// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package elasticache

import (
	"context"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	awstypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/hashicorp/terraform-provider-aws/internal/conns"
	"github.com/hashicorp/terraform-provider-aws/internal/errs"
	"github.com/hashicorp/terraform-provider-aws/internal/errs/sdkdiag"
	"github.com/hashicorp/terraform-provider-aws/internal/flex"
	"github.com/hashicorp/terraform-provider-aws/internal/retry"
	"github.com/hashicorp/terraform-provider-aws/internal/sdkv2"
	tfslices "github.com/hashicorp/terraform-provider-aws/internal/slices"
	tftags "github.com/hashicorp/terraform-provider-aws/internal/tags"
	"github.com/hashicorp/terraform-provider-aws/internal/tfresource"
	"github.com/hashicorp/terraform-provider-aws/internal/verify"
	"github.com/hashicorp/terraform-provider-aws/names"
)

// @SDKResource("aws_elasticache_user_group", name="User Group")
// @Tags(identifierAttribute="arn")
func resourceUserGroup() *schema.Resource {
	return &schema.Resource{
		CreateWithoutTimeout: resourceUserGroupCreate,
		ReadWithoutTimeout:   resourceUserGroupRead,
		UpdateWithoutTimeout: resourceUserGroupUpdate,
		DeleteWithoutTimeout: resourceUserGroupDelete,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			names.AttrARN: {
				Type:     schema.TypeString,
				Computed: true,
			},
			names.AttrEngine: {
				Type:     schema.TypeString,
				Required: true,
				ValidateDiagFunc: validation.AllDiag(
					validation.ToDiagFunc(validation.StringInSlice([]string{engineRedis, engineValkey}, true)),
					verify.CaseInsensitiveMatchDeprecation([]string{engineRedis, engineValkey}),
				),
				DiffSuppressFunc: sdkv2.SuppressEquivalentStringCaseInsensitive,
			},
			names.AttrTags:    tftags.TagsSchema(),
			names.AttrTagsAll: tftags.TagsSchemaComputed(),
			"user_group_id": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"user_ids": {
				Type:     schema.TypeSet,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
		},
	}
}

func resourceUserGroupCreate(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).ElastiCacheClient(ctx)
	partition := meta.(*conns.AWSClient).Partition(ctx)

	userGroupID := d.Get("user_group_id").(string)
	input := &elasticache.CreateUserGroupInput{
		Engine:      aws.String(d.Get(names.AttrEngine).(string)),
		Tags:        getTagsIn(ctx),
		UserGroupId: aws.String(userGroupID),
	}

	if v, ok := d.GetOk("user_ids"); ok && v.(*schema.Set).Len() > 0 {
		input.UserIds = flex.ExpandStringValueSet(v.(*schema.Set))
	}

	output, err := conn.CreateUserGroup(ctx, input)

	// Some partitions (e.g. ISO) may not support tag-on-create.
	if input.Tags != nil && errs.IsUnsupportedOperationInPartitionError(partition, err) {
		input.Tags = nil

		output, err = conn.CreateUserGroup(ctx, input)
	}

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "creating ElastiCache User Group (%s): %s", userGroupID, err)
	}

	d.SetId(aws.ToString(output.UserGroupId))

	if _, err := waitUserGroupCreated(ctx, conn, d.Id(), d.Timeout(schema.TimeoutCreate)); err != nil {
		return sdkdiag.AppendErrorf(diags, "waiting for ElastiCache User Group (%s) create: %s", d.Id(), err)
	}

	// For partitions not supporting tag-on-create, attempt tag after create.
	if tags := getTagsIn(ctx); input.Tags == nil && len(tags) > 0 {
		err := createTags(ctx, conn, aws.ToString(output.ARN), tags)

		// If default tags only, continue. Otherwise, error.
		if v, ok := d.GetOk(names.AttrTags); (!ok || len(v.(map[string]any)) == 0) && errs.IsUnsupportedOperationInPartitionError(partition, err) {
			return append(diags, resourceUserGroupRead(ctx, d, meta)...)
		}

		if err != nil {
			return sdkdiag.AppendErrorf(diags, "setting ElastiCache User Group (%s) tags: %s", d.Id(), err)
		}
	}

	return append(diags, resourceUserGroupRead(ctx, d, meta)...)
}

func resourceUserGroupRead(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).ElastiCacheClient(ctx)

	userGroup, err := findUserGroupByID(ctx, conn, d.Id())

	if !d.IsNewResource() && retry.NotFound(err) {
		log.Printf("[WARN] ElastiCache User Group (%s) not found, removing from state", d.Id())
		d.SetId("")
		return diags
	}

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "reading ElastiCache User Group (%s): %s", d.Id(), err)
	}

	d.Set(names.AttrARN, userGroup.ARN)
	d.Set(names.AttrEngine, userGroup.Engine)
	d.Set("user_ids", userGroup.UserIds)
	d.Set("user_group_id", userGroup.UserGroupId)

	return diags
}

func resourceUserGroupUpdate(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).ElastiCacheClient(ctx)

	if d.HasChangesExcept(names.AttrTags, names.AttrTagsAll) {
		input := &elasticache.ModifyUserGroupInput{
			UserGroupId: aws.String(d.Get("user_group_id").(string)),
		}

		if d.HasChange(names.AttrEngine) {
			input.Engine = aws.String(d.Get(names.AttrEngine).(string))
		}

		if d.HasChange("user_ids") {
			o, n := d.GetChange("user_ids")
			add, del := n.(*schema.Set).Difference(o.(*schema.Set)), o.(*schema.Set).Difference(n.(*schema.Set))

			if add.Len() > 0 {
				input.UserIdsToAdd = flex.ExpandStringValueSet(add)
			}
			if del.Len() > 0 {
				input.UserIdsToRemove = flex.ExpandStringValueSet(del)
			}
		}

		_, err := conn.ModifyUserGroup(ctx, input)

		if err != nil {
			return sdkdiag.AppendErrorf(diags, "updating ElastiCache User Group (%q): %s", d.Id(), err)
		}

		if _, err := waitUserGroupUpdated(ctx, conn, d.Id(), d.Timeout(schema.TimeoutUpdate)); err != nil {
			return sdkdiag.AppendErrorf(diags, "waiting for ElastiCache User Group (%s) update: %s", d.Id(), err)
		}
	}

	return append(diags, resourceUserGroupRead(ctx, d, meta)...)
}

func resourceUserGroupDelete(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).ElastiCacheClient(ctx)

	log.Printf("[INFO] Deleting ElastiCache User Group: %s", d.Id())
	_, err := conn.DeleteUserGroup(ctx, &elasticache.DeleteUserGroupInput{
		UserGroupId: aws.String(d.Id()),
	})

	if errs.IsA[*awstypes.UserGroupNotFoundFault](err) {
		return diags
	}

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "deleting ElastiCache User Group (%s): %s", d.Id(), err)
	}

	if _, err := waitUserGroupDeleted(ctx, conn, d.Id(), d.Timeout(schema.TimeoutDelete)); err != nil {
		return sdkdiag.AppendErrorf(diags, "waiting for ElastiCache User Group (%s) delete: %s", d.Id(), err)
	}

	return diags
}

func findUserGroupByID(ctx context.Context, conn *elasticache.Client, id string) (*awstypes.UserGroup, error) {
	input := &elasticache.DescribeUserGroupsInput{
		UserGroupId: aws.String(id),
	}

	return findUserGroup(ctx, conn, input, tfslices.PredicateTrue[*awstypes.UserGroup]())
}

func findUserGroup(ctx context.Context, conn *elasticache.Client, input *elasticache.DescribeUserGroupsInput, filter tfslices.Predicate[*awstypes.UserGroup]) (*awstypes.UserGroup, error) {
	output, err := findUserGroups(ctx, conn, input, filter)

	if err != nil {
		return nil, err
	}

	return tfresource.AssertSingleValueResult(output)
}

func findUserGroups(ctx context.Context, conn *elasticache.Client, input *elasticache.DescribeUserGroupsInput, filter tfslices.Predicate[*awstypes.UserGroup]) ([]awstypes.UserGroup, error) {
	var output []awstypes.UserGroup

	pages := elasticache.NewDescribeUserGroupsPaginator(conn, input)

	for pages.HasMorePages() {
		page, err := pages.NextPage(ctx)

		if errs.IsA[*awstypes.UserGroupNotFoundFault](err) {
			return nil, &retry.NotFoundError{
				LastError: err,
			}
		}

		if err != nil {
			return nil, err
		}

		for _, v := range page.UserGroups {
			if filter(&v) {
				output = append(output, v)
			}
		}
	}

	return output, nil
}

func statusUserGroup(conn *elasticache.Client, id string) retry.StateRefreshFunc {
	return func(ctx context.Context) (any, string, error) {
		output, err := findUserGroupByID(ctx, conn, id)

		if retry.NotFound(err) {
			return nil, "", nil
		}

		if err != nil {
			return nil, "", err
		}

		return output, aws.ToString(output.Status), nil
	}
}

const (
	userGroupStatusActive    = "active"
	userGroupStatusCreating  = "creating"
	userGroupStatusDeleting  = "deleting"
	userGroupStatusModifying = "modifying"
)

func waitUserGroupCreated(ctx context.Context, conn *elasticache.Client, id string, timeout time.Duration) (*awstypes.UserGroup, error) {
	stateConf := &retry.StateChangeConf{
		Pending:    []string{userGroupStatusCreating, userGroupStatusModifying},
		Target:     []string{userGroupStatusActive},
		Refresh:    statusUserGroup(conn, id),
		Timeout:    timeout,
		MinTimeout: 10 * time.Second,
		Delay:      30 * time.Second,
	}

	outputRaw, err := stateConf.WaitForStateContext(ctx)

	if output, ok := outputRaw.(*awstypes.UserGroup); ok {
		return output, err
	}

	return nil, err
}

func waitUserGroupUpdated(ctx context.Context, conn *elasticache.Client, id string, timeout time.Duration) (*awstypes.UserGroup, error) { //nolint:unparam
	stateConf := &retry.StateChangeConf{
		Pending:    []string{userGroupStatusModifying},
		Target:     []string{userGroupStatusActive},
		Refresh:    statusUserGroup(conn, id),
		Timeout:    timeout,
		MinTimeout: 10 * time.Second,
		Delay:      30 * time.Second,
	}

	outputRaw, err := stateConf.WaitForStateContext(ctx)

	if output, ok := outputRaw.(*awstypes.UserGroup); ok {
		return output, err
	}

	return nil, err
}

func waitUserGroupDeleted(ctx context.Context, conn *elasticache.Client, id string, timeout time.Duration) (*awstypes.UserGroup, error) {
	stateConf := &retry.StateChangeConf{
		Pending:    []string{userGroupStatusDeleting},
		Target:     []string{},
		Refresh:    statusUserGroup(conn, id),
		Timeout:    timeout,
		MinTimeout: 10 * time.Second,
		Delay:      30 * time.Second,
	}

	outputRaw, err := stateConf.WaitForStateContext(ctx)

	if output, ok := outputRaw.(*awstypes.UserGroup); ok {
		return output, err
	}

	return nil, err
}
