// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package cloudfrontkeyvaluestore

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/service/cloudfrontkeyvaluestore"
	awstypes "github.com/aws/aws-sdk-go-v2/service/cloudfrontkeyvaluestore/types"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"
	"github.com/hashicorp/terraform-provider-aws/internal/conns"
	"github.com/hashicorp/terraform-provider-aws/internal/errs"
	"github.com/hashicorp/terraform-provider-aws/internal/errs/fwdiag"
	"github.com/hashicorp/terraform-provider-aws/internal/flex"
	"github.com/hashicorp/terraform-provider-aws/internal/framework"
	fwflex "github.com/hashicorp/terraform-provider-aws/internal/framework/flex"
	fwtypes "github.com/hashicorp/terraform-provider-aws/internal/framework/types"
	"github.com/hashicorp/terraform-provider-aws/internal/tfresource"
	"github.com/hashicorp/terraform-provider-aws/names"
)

// @FrameworkResource("aws_cloudfrontkeyvaluestore_key", name="Key")
// @IdentityAttribute("key_value_store_arn")
// @IdentityAttribute("key")
// @WrappedImport(false)
// @Testing(identityTest=false)
func newKeyResource(_ context.Context) (resource.ResourceWithConfigure, error) {
	r := &keyResource{}

	return r, nil
}

type keyResource struct {
	framework.ResourceWithModel[keyResourceModel]
}

func (r *keyResource) Schema(ctx context.Context, request resource.SchemaRequest, response *resource.SchemaResponse) {
	response.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			names.AttrID: framework.IDAttributeDeprecatedNoReplacement(),
			names.AttrKey: schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The key to put.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"key_value_store_arn": schema.StringAttribute{
				CustomType:          fwtypes.ARNType,
				Required:            true,
				MarkdownDescription: "The Amazon Resource Name (ARN) of the Key Value Store.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"total_size_in_bytes": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Total size of the Key Value Store in bytes.",
			},
			names.AttrValue: schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The value to put.",
			},
		},
	}
}

func (r *keyResource) Create(ctx context.Context, request resource.CreateRequest, response *resource.CreateResponse) {
	var data keyResourceModel
	response.Diagnostics.Append(request.Plan.Get(ctx, &data)...)
	if response.Diagnostics.HasError() {
		return
	}

	conn := r.Meta().CloudFrontKeyValueStoreClient(ctx)

	kvsARN := data.KvsARN.ValueString()

	// Adding a key changes the etag of the key value store.
	// Use a mutex serialize actions
	mutexKey := kvsARN
	conns.GlobalMutexKV.Lock(mutexKey)
	defer conns.GlobalMutexKV.Unlock(mutexKey)

	etag, err := findETagByARN(ctx, conn, kvsARN)

	if err != nil {
		response.Diagnostics.AddError(fmt.Sprintf("reading CloudFront KeyValueStore ETag (%s)", kvsARN), err.Error())

		return
	}

	input := &cloudfrontkeyvaluestore.PutKeyInput{}
	response.Diagnostics.Append(fwflex.Expand(ctx, data, input)...)
	if response.Diagnostics.HasError() {
		return
	}

	// Additional fields.
	input.IfMatch = etag

	output, err := conn.PutKey(ctx, input)

	if err != nil {
		response.Diagnostics.AddError(fmt.Sprintf("creating CloudFront KeyValueStore (%s) Key (%s)", kvsARN, data.Key.ValueString()), err.Error())

		return
	}

	// Set values for unknowns.
	data.TotalSizeInBytes = fwflex.Int64ToFramework(ctx, output.TotalSizeInBytes)
	id, err := data.setID()
	if err != nil {
		response.Diagnostics.AddError(fmt.Sprintf("creating CloudFront KeyValueStore (%s) Key (%s)", kvsARN, data.Key.ValueString()), err.Error())
		return
	}
	data.ID = types.StringValue(id)

	response.Diagnostics.Append(response.State.Set(ctx, data)...)
}

func (r *keyResource) Read(ctx context.Context, request resource.ReadRequest, response *resource.ReadResponse) {
	var data keyResourceModel
	response.Diagnostics.Append(request.State.Get(ctx, &data)...)
	if response.Diagnostics.HasError() {
		return
	}

	conn := r.Meta().CloudFrontKeyValueStoreClient(ctx)

	output, err := findKeyByTwoPartKey(ctx, conn, data.KvsARN.ValueString(), data.Key.ValueString())

	if tfresource.NotFound(err) {
		response.Diagnostics.Append(fwdiag.NewResourceNotFoundWarningDiagnostic(err))
		response.State.RemoveResource(ctx)

		return
	}

	if err != nil {
		response.Diagnostics.AddError(fmt.Sprintf("reading CloudFront KeyValueStore Key (%s)", data.ID.ValueString()), err.Error())

		return
	}

	// Set attributes for import.
	response.Diagnostics.Append(fwflex.Flatten(ctx, output, &data)...)
	if response.Diagnostics.HasError() {
		return
	}

	response.Diagnostics.Append(response.State.Set(ctx, &data)...)
}

func (r *keyResource) Update(ctx context.Context, request resource.UpdateRequest, response *resource.UpdateResponse) {
	var old, new keyResourceModel
	response.Diagnostics.Append(request.State.Get(ctx, &old)...)
	if response.Diagnostics.HasError() {
		return
	}
	response.Diagnostics.Append(request.Plan.Get(ctx, &new)...)
	if response.Diagnostics.HasError() {
		return
	}

	conn := r.Meta().CloudFrontKeyValueStoreClient(ctx)

	if !new.Value.Equal(old.Value) {
		kvsARN := new.KvsARN.ValueString()

		// Updating a key changes the etag of the key value store.
		// Use a mutex serialize actions
		mutexKey := kvsARN
		conns.GlobalMutexKV.Lock(mutexKey)
		defer conns.GlobalMutexKV.Unlock(mutexKey)

		etag, err := findETagByARN(ctx, conn, kvsARN)

		if err != nil {
			response.Diagnostics.AddError(fmt.Sprintf("reading CloudFront KeyValueStore ETag (%s)", kvsARN), err.Error())

			return
		}

		input := &cloudfrontkeyvaluestore.PutKeyInput{}
		response.Diagnostics.Append(fwflex.Expand(ctx, new, input)...)
		if response.Diagnostics.HasError() {
			return
		}

		// Additional fields.
		input.IfMatch = etag

		output, err := conn.PutKey(ctx, input)

		if err != nil {
			response.Diagnostics.AddError(fmt.Sprintf("updating CloudFront KeyValueStore (%s) Key (%s)", kvsARN, new.Key.ValueString()), err.Error())

			return
		}

		// Set values for unknowns.
		new.TotalSizeInBytes = fwflex.Int64ToFramework(ctx, output.TotalSizeInBytes)
	}

	response.Diagnostics.Append(response.State.Set(ctx, &new)...)
}

func (r *keyResource) Delete(ctx context.Context, request resource.DeleteRequest, response *resource.DeleteResponse) {
	var data keyResourceModel
	response.Diagnostics.Append(request.State.Get(ctx, &data)...)
	if response.Diagnostics.HasError() {
		return
	}

	conn := r.Meta().CloudFrontKeyValueStoreClient(ctx)

	kvsARN := data.KvsARN.ValueString()

	// Deleting a key changes the etag of the key value store.
	// Use a mutex serialize actions
	mutexKey := kvsARN
	conns.GlobalMutexKV.Lock(mutexKey)
	defer conns.GlobalMutexKV.Unlock(mutexKey)

	etag, err := findETagByARN(ctx, conn, kvsARN)

	if err != nil {
		response.Diagnostics.AddError(fmt.Sprintf("reading CloudFront KeyValueStore ETag (%s)", kvsARN), err.Error())

		return
	}

	input := cloudfrontkeyvaluestore.DeleteKeyInput{
		IfMatch: etag,
		Key:     fwflex.StringFromFramework(ctx, data.Key),
		KvsARN:  fwflex.StringFromFramework(ctx, data.KvsARN),
	}
	_, err = conn.DeleteKey(ctx, &input)

	if errs.IsA[*awstypes.ResourceNotFoundException](err) {
		return
	}

	if err != nil {
		response.Diagnostics.AddError(fmt.Sprintf("deleting CloudFront KeyValueStore Key (%s)", data.ID.ValueString()), err.Error())

		return
	}
}

func findKeyByTwoPartKey(ctx context.Context, conn *cloudfrontkeyvaluestore.Client, kvsARN, key string) (*cloudfrontkeyvaluestore.GetKeyOutput, error) {
	input := &cloudfrontkeyvaluestore.GetKeyInput{
		Key:    aws.String(key),
		KvsARN: aws.String(kvsARN),
	}

	output, err := conn.GetKey(ctx, input)

	if errs.IsA[*awstypes.ResourceNotFoundException](err) {
		return nil, &retry.NotFoundError{
			LastError:   err,
			LastRequest: input,
		}
	}

	if err != nil {
		return nil, err
	}

	if output == nil || output.Key == nil {
		return nil, tfresource.NewEmptyResultError(input)
	}

	return output, nil
}

func findETagByARN(ctx context.Context, conn *cloudfrontkeyvaluestore.Client, arn string) (*string, error) {
	input := &cloudfrontkeyvaluestore.DescribeKeyValueStoreInput{
		KvsARN: aws.String(arn),
	}

	output, err := conn.DescribeKeyValueStore(ctx, input)

	if errs.IsA[*awstypes.ResourceNotFoundException](err) {
		return nil, &retry.NotFoundError{
			LastError:   err,
			LastRequest: input,
		}
	}

	if err != nil {
		return nil, err
	}

	if output == nil || output.ETag == nil {
		return nil, tfresource.NewEmptyResultError(input)
	}

	return output.ETag, nil
}

type keyResourceModel struct {
	ID               types.String `tfsdk:"id"`
	Key              types.String `tfsdk:"key"`
	KvsARN           fwtypes.ARN  `tfsdk:"key_value_store_arn"`
	TotalSizeInBytes types.Int64  `tfsdk:"total_size_in_bytes"`
	Value            types.String `tfsdk:"value"`
}

const (
	keyResourceIDPartCount = 2
)

func (data *keyResourceModel) setID() (string, error) {
	parts := []string{
		data.KvsARN.ValueString(),
		data.Key.ValueString(),
	}

	return flex.FlattenResourceId(parts, keyResourceIDPartCount, false)
}

func (r *keyResource) ImportState(ctx context.Context, request resource.ImportStateRequest, response *resource.ImportStateResponse) {
	// Import-by-id case
	if request.ID != "" {
		id := request.ID
		parts, err := flex.ExpandResourceId(id, keyResourceIDPartCount, false)
		if err != nil {
			response.Diagnostics.AddError(
				"Parsing Import ID",
				err.Error(),
			)
			return
		}

		_, err = arn.Parse(parts[0])
		if err != nil {
			response.Diagnostics.AddError(
				"Parsing Import ID",
				err.Error(),
			)
			return
		}

		response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("key_value_store_arn"), parts[0])...)
		response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root(names.AttrKey), parts[1])...)
		response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root(names.AttrID), request.ID)...) // nosemgrep:ci.semgrep.framework.import-state-passthrough-id

		return
	}

	if identity := request.Identity; identity != nil {
		var arn string
		identity.GetAttribute(ctx, path.Root("key_value_store_arn"), &arn)

		response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("key_value_store_arn"), arn)...)
		if response.Diagnostics.HasError() {
			return
		}

		var key string
		identity.GetAttribute(ctx, path.Root(names.AttrKey), &key)

		response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root(names.AttrKey), key)...)
		if response.Diagnostics.HasError() {
			return
		}

		parts := []string{
			arn,
			key,
		}
		id, _ := flex.FlattenResourceId(parts, keyResourceIDPartCount, false)

		response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root(names.AttrID), id)...)
	}
}
