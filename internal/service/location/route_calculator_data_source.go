// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package location

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/hashicorp/terraform-provider-aws/internal/conns"
	"github.com/hashicorp/terraform-provider-aws/internal/errs/sdkdiag"
	tftags "github.com/hashicorp/terraform-provider-aws/internal/tags"
	"github.com/hashicorp/terraform-provider-aws/names"
)

// @SDKDataSource("aws_location_route_calculator", name="Route Calculator")
func DataSourceRouteCalculator() *schema.Resource {
	return &schema.Resource{
		ReadWithoutTimeout: dataSourceRouteCalculatorRead,

		Schema: map[string]*schema.Schema{
			"calculator_arn": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"calculator_name": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.StringLenBetween(1, 100),
			},
			names.AttrCreateTime: {
				Type:     schema.TypeString,
				Computed: true,
			},
			"data_source": {
				Type:     schema.TypeString,
				Computed: true,
			},
			names.AttrDescription: {
				Type:     schema.TypeString,
				Computed: true,
			},
			"update_time": {
				Type:     schema.TypeString,
				Computed: true,
			},
			names.AttrTags: tftags.TagsSchemaComputed(),
		},
	}
}

func dataSourceRouteCalculatorRead(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics

	conn := meta.(*conns.AWSClient).LocationClient(ctx)

	out, err := findRouteCalculatorByName(ctx, conn, d.Get("calculator_name").(string))
	if err != nil {
		return sdkdiag.AppendErrorf(diags, "reading Location Service Route Calculator (%s): %s", d.Get("calculator_name").(string), err)
	}

	if out == nil {
		return sdkdiag.AppendErrorf(diags, "reading Location Service Route Calculator (%s): empty response", d.Get("calculator_name").(string))
	}

	d.SetId(aws.ToString(out.CalculatorName))
	d.Set("calculator_arn", out.CalculatorArn)
	d.Set("calculator_name", out.CalculatorName)
	d.Set(names.AttrCreateTime, aws.ToTime(out.CreateTime).Format(time.RFC3339))
	d.Set("data_source", out.DataSource)
	d.Set(names.AttrDescription, out.Description)
	d.Set("update_time", aws.ToTime(out.UpdateTime).Format(time.RFC3339))

	ignoreTagsConfig := meta.(*conns.AWSClient).IgnoreTagsConfig(ctx)

	if err := d.Set(names.AttrTags, keyValueTags(ctx, out.Tags).IgnoreAWS().IgnoreConfig(ignoreTagsConfig).Map()); err != nil {
		return sdkdiag.AppendErrorf(diags, "listing tags for Location Service Route Calculator (%s): %s", d.Id(), err)
	}

	return diags
}
