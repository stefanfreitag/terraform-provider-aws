---
subcategory: "Route 53"
layout: "aws"
page_title: "AWS: aws_route53_records_exclusive"
description: |-
  Terraform resource for maintaining exclusive management of resource record sets defined in an AWS Route53 hosted zone.
---
# Resource: aws_route53_records_exclusive

Terraform resource for maintaining exclusive management of resource record sets defined in an AWS Route53 hosted zone.

!> This resource takes exclusive ownership over resource record sets defined in a hosted zone. This includes removal of record sets which are not explicitly configured. To prevent persistent drift, ensure any `aws_route53_record` resources managed alongside this resource have an equivalent `resource_record_set` argument.

~> Destruction of this resource means Terraform will no longer manage reconciliation of the configured resource record sets. It __will not__ delete the configured record sets from the hosted zone.

~> The default `NS` and `SOA` records created during provisioning of the Route53 Zone __should not be included__ in this resource definition. Adding them will cause persistent drift as the read operation is explicitly configured to ignore writing them to state.

## Example Usage

### Basic Usage

```terraform
resource "aws_route53_zone" "example" {
  name          = "example.com"
  force_destroy = true
}

resource "aws_route53_records_exclusive" "test" {
  zone_id = aws_route53_zone.test.zone_id

  resource_record_set {
    name = "subdomain.example.com"
    type = "A"
    ttl  = "30"

    resource_records {
      value = "127.0.0.1"
    }
    resource_records {
      value = "127.0.0.27"
    }
  }
}
```

### Disallow Record Sets

To automatically remove any configured record sets, omit a `resource_record_set` block.

~> This will not __prevent__ record sets from being defined in a hosted zone via Terraform (or any other interface). This resource enables bringing record set definitions into a configured state, however, this reconciliation happens only when `apply` is proactively run.

```terraform
resource "aws_route53_records_exclusive" "test" {
  zone_id = aws_route53_zone.test.zone_id
}
```

## Argument Reference

The following arguments are required:

* `zone_id` - (Required) ID of the hosted zone containing the resource record sets.

The following arguments are optional:

* `resource_record_set` - (Optional) A list of all resource record sets associated with the hosted zone.
See [`resource_record_set`](#resource_record_set) below.

### `resource_record_set`

The following arguments are required:

* `name` - (Required) Name of the record.
* `type` - (Required) Record type.
Valid values are `A`, `AAAA`, `CAA`, `CNAME`, `DS`, `MX`, `NAPTR`, `NS`, `PTR`, `SOA`, `SPF`, `SRV`, `TXT`, `TLSA`, `SSHFP`, `SVCB`, and `HTTPS`.

The following arguments are optional:

~> Exactly one of `resource_records` or `alias_target` must be specified.

* `alias_target` - (Optional) Alias target block.
See [`alias_target`](#alias_target) below.
* `cidr_routing_policy` - (Optional) CIDR routing configuration block.
See [`cidr_routing_config`](#cidr_routing_config) below.
* `failover` - (Optional) Type of failover resource record.
Valid values are `PRIMARY` and `SECONDARY`.
See the [AWS documentation on DNS failover](https://docs.aws.amazon.com/Route53/latest/DeveloperGuide/dns-failover.html) for additional details.
* `geolocation` - (Optional) Geolocation block to control how Amazon Route 53 responds to DNS queries based on the geographic origin of the query.
See [`geolocation`](#geolocation) below.
* `geoproximity_location` - (Optional) Geoproximity location block.
See [`geoproximity_location`](#geoproximity_location) below.
* `health_check_id` - (Optional) Health check the record should be associated with.
* `multivalue_answer` - (Optional) Set to `true` to indicate this record is a multivalue answer record and traffic should be routed approximately randomly to multiple resources.
* `region` - (Optional) AWS region of the resource this record set refers to.
Must be a valid AWS region name.
See the [AWS documentation](http://docs.aws.amazon.com/Route53/latest/DeveloperGuide/routing-policy.html#routing-policy-latency) on latency based routing for additional details.
* `resource_records` - (Optional, Required for non-alias records) Information about the resource records to act upon.
See [`resource_records`](#resource_records) below.
* `set_identifier` - (Optional) An identifier that differentiates among multiple resource record sets that have the same combination of name and type.
Required if using `cidr_routing_config`, `failover`, `geolocation`,`geoproximity_location`, `multivalue_answer`, `region`, or `weight`.
* `traffic_policy_instance_id` - (Optional) ID of the traffic policy instance that Route 53 created this resource record set for.
To delete the resource record set that is associated with a traffic policy instance, use the `DeleteTrafficPolicyInstance` API.
Route 53 will delete the resource record set automatically.
If the resource record set is deleted via `ChangeResourceRecordSets` (the API underpinning this Terraform resource), Route 53 doesn't automatically delete the traffic policy instance, and you'll continue to be charged for it.
* `ttl` - (Optional, Required for non-alias records) Resource record cache time to live (TTL), in seconds.
* `weight` - (Optional) Among resource record sets that have the same combination of DNS name and type, a value that determines the proportion of DNS queries that Amazon Route 53 responds to using the current resource record set.

### `alias_target`

* `dns_name` - (Required) DNS domain name for another resource record set in this hosted zone.
* `evaluate_target_health` - (Required) Set to `true` if you want Route 53 to determine whether to respond to DNS queries using this resource record set by checking the health of the resource record set. Some resources have special requirements, see [the AWS documentation](https://docs.aws.amazon.com/Route53/latest/DeveloperGuide/resource-record-sets-values.html#rrsets-values-alias-evaluate-target-health) for additional details.
* `hosted_zone_id` - (Required) Hosted zone ID for a CloudFront distribution, S3 bucket, ELB, AWS Global Accelerator, or Route 53 hosted zone. See [`resource_elb.zone_id`](/docs/providers/aws/r/elb.html#zone_id) for an example.

### `cidr_routing_config`

* `collection_id` - (Required) CIDR collection ID.
See the [`aws_route53_cidr_collection` resource](route53_cidr_collection.html) for more details.
* `location_name` - (Required) CIDR collection location name.
See the [`aws_route53_cidr_location` resource](route53_cidr_location.html) for more details.
A `location_name` with an asterisk `"*"` can be used to create a default CIDR record.
`collection_id` is still required for a default record.

### `geolocation`

~> One of `continent` or `country` must be specified.

* `continent` - (Optional) Two-letter continent code.
See the [AWS documentation](http://docs.aws.amazon.com/Route53/latest/APIReference/API_GetGeoLocation.html) for valid values.
* `country` - (Optional) Two-letter country code.
See the ISO standard linked from the [AWS documentation](http://docs.aws.amazon.com/Route53/latest/APIReference/API_GetGeoLocation.html) for valid values.
* `subdivision` - (Optional) Subdivision code.

### `geoproximity_location`

* `aws_region` - (Optional) AWS region of the resource where DNS traffic is directed to.
* `bias` - (Optional) Increases or decreases the size of the geographic region from which Route 53 routes traffic to a resource.
To expand the size of the geographic region from which Route 53 routes traffic to a resource, specify a positive integer from `1` to `99`.
To shrink the size of the geographic region from which Route 53 routes traffic to a resource, specify a negative bias of `-1` to `-99`.
See the [AWS documentation](https://docs.aws.amazon.com/Route53/latest/DeveloperGuide/routing-policy-geoproximity.html) for additional details.
* `coordinates` - (Optional) Coordinates for a geoproximity resource record.
See [`coordinates`](#coordinates) below.
* `local_zone_group` - (Optional) AWS local zone group.
Identify the Local Zones Group for a specific Local Zone by using the [`describe-availability-zones` CLI command](https://docs.aws.amazon.com/cli/latest/reference/ec2/describe-availability-zones.html).

#### `coordinates`

* `latitude` - (Required) A coordinate of the north–south position of a geographic point on the surface of the Earth (`-90` - `90`).
* `longitude` - (Required) A coordinate of the east–west position of a geographic point on the surface of the Earth (`-180` - `180`).

### `resource_records`

* `value` - (Required) DNS record value.

## Attribute Reference

This resource exports no additional attributes.

## Timeouts

[Configuration options](https://developer.hashicorp.com/terraform/language/resources/syntax#operation-timeouts):

* `create` - (Default `45m`)
* `update` - (Default `45m`)

## Import

In Terraform v1.5.0 and later, use an [`import` block](https://developer.hashicorp.com/terraform/language/import) to import Route 53 Records Exclusive using the `zone_id`. For example:

```terraform
import {
  to = aws_route53_records_exclusive.example
  id = "ABCD1234"
}
```

Using `terraform import`, import Route 53 Records Exclusive using the `zone_id`. For example:

```console
% terraform import aws_route53_records_exclusive.example ABCD1234
```
