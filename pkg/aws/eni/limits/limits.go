// Copyright 2019-2020 Authors of Cilium
// Copyright 2017 Lyft, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package limits

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	ec2shim "github.com/cilium/cilium/pkg/aws/ec2"
	ipamTypes "github.com/cilium/cilium/pkg/ipam/types"
	"github.com/cilium/cilium/pkg/lock"

	"github.com/aws/aws-sdk-go-v2/aws"
)

var limitsOnce sync.Once

// limit contains limits for adapter count and addresses. The mappings will be
// updated from agent configuration at bootstrap time.
//
// Source: https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/using-eni.html?shortFooter=true#AvailableIpPerENI
//
// Generated using the following command (requires AWS cli & jq):
// AWS_REGION=us-east-1 aws ec2 describe-instance-types | jq -r '.InstanceTypes[] |
// "\"\(.InstanceType)\": {Adapters: \(.NetworkInfo.MaximumNetworkInterfaces), IPv4: \(.NetworkInfo.Ipv4AddressesPerInterface), IPv6: \(.NetworkInfo.Ipv6AddressesPerInterface), HypervisorType: \"\(.Hypervisor)\"},"' \
// | sort | sed "s/null//"
var limits struct {
	lock.RWMutex
	m map[string]ipamTypes.Limits
}

func populateStaticENILimits() {
	limits.m = map[string]ipamTypes.Limits{
		"a1.2xlarge":        {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"a1.4xlarge":        {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"a1.large":          {Adapters: 3, IPv4: 10, IPv6: 10, HypervisorType: "nitro"},
		"a1.medium":         {Adapters: 2, IPv4: 4, IPv6: 4, HypervisorType: "nitro"},
		"a1.metal":          {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: ""},
		"a1.xlarge":         {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"c1.medium":         {Adapters: 2, IPv4: 6, IPv6: 0, HypervisorType: "xen"},
		"c1.xlarge":         {Adapters: 4, IPv4: 15, IPv6: 0, HypervisorType: "xen"},
		"c3.2xlarge":        {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "xen"},
		"c3.4xlarge":        {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "xen"},
		"c3.8xlarge":        {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "xen"},
		"c3.large":          {Adapters: 3, IPv4: 10, IPv6: 10, HypervisorType: "xen"},
		"c3.xlarge":         {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "xen"},
		"c4.2xlarge":        {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "xen"},
		"c4.4xlarge":        {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "xen"},
		"c4.8xlarge":        {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "xen"},
		"c4.large":          {Adapters: 3, IPv4: 10, IPv6: 10, HypervisorType: "xen"},
		"c4.xlarge":         {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "xen"},
		"c5.12xlarge":       {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"c5.18xlarge":       {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"c5.24xlarge":       {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"c5.2xlarge":        {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"c5.4xlarge":        {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"c5.9xlarge":        {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"c5.large":          {Adapters: 3, IPv4: 10, IPv6: 10, HypervisorType: "nitro"},
		"c5.metal":          {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: ""},
		"c5.xlarge":         {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"c5a.12xlarge":      {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"c5a.16xlarge":      {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"c5a.24xlarge":      {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"c5a.2xlarge":       {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"c5a.4xlarge":       {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"c5a.8xlarge":       {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"c5a.large":         {Adapters: 3, IPv4: 10, IPv6: 10, HypervisorType: "nitro"},
		"c5a.xlarge":        {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"c5ad.12xlarge":     {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"c5ad.16xlarge":     {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"c5ad.24xlarge":     {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"c5ad.2xlarge":      {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"c5ad.4xlarge":      {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"c5ad.8xlarge":      {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"c5ad.large":        {Adapters: 3, IPv4: 10, IPv6: 10, HypervisorType: "nitro"},
		"c5ad.xlarge":       {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"c5d.12xlarge":      {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"c5d.18xlarge":      {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"c5d.24xlarge":      {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"c5d.2xlarge":       {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"c5d.4xlarge":       {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"c5d.9xlarge":       {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"c5d.large":         {Adapters: 3, IPv4: 10, IPv6: 10, HypervisorType: "nitro"},
		"c5d.metal":         {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: ""},
		"c5d.xlarge":        {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"c5n.18xlarge":      {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"c5n.2xlarge":       {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"c5n.4xlarge":       {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"c5n.9xlarge":       {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"c5n.large":         {Adapters: 3, IPv4: 10, IPv6: 10, HypervisorType: "nitro"},
		"c5n.metal":         {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: ""},
		"c5n.xlarge":        {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"c6a.12xlarge":      {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"c6a.16xlarge":      {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"c6a.24xlarge":      {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"c6a.2xlarge":       {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"c6a.32xlarge":      {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"c6a.48xlarge":      {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"c6a.4xlarge":       {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"c6a.8xlarge":       {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"c6a.large":         {Adapters: 3, IPv4: 10, IPv6: 10, HypervisorType: "nitro"},
		"c6a.xlarge":        {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"c6g.12xlarge":      {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"c6g.16xlarge":      {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"c6g.2xlarge":       {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"c6g.4xlarge":       {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"c6g.8xlarge":       {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"c6g.large":         {Adapters: 3, IPv4: 10, IPv6: 10, HypervisorType: "nitro"},
		"c6g.medium":        {Adapters: 2, IPv4: 4, IPv6: 4, HypervisorType: "nitro"},
		"c6g.metal":         {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: ""},
		"c6g.xlarge":        {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"c6gd.12xlarge":     {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"c6gd.16xlarge":     {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"c6gd.2xlarge":      {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"c6gd.4xlarge":      {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"c6gd.8xlarge":      {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"c6gd.large":        {Adapters: 3, IPv4: 10, IPv6: 10, HypervisorType: "nitro"},
		"c6gd.medium":       {Adapters: 2, IPv4: 4, IPv6: 4, HypervisorType: "nitro"},
		"c6gd.metal":        {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: ""},
		"c6gd.xlarge":       {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"c6gn.12xlarge":     {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"c6gn.16xlarge":     {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"c6gn.2xlarge":      {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"c6gn.4xlarge":      {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"c6gn.8xlarge":      {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"c6gn.large":        {Adapters: 3, IPv4: 10, IPv6: 10, HypervisorType: "nitro"},
		"c6gn.medium":       {Adapters: 2, IPv4: 4, IPv6: 4, HypervisorType: "nitro"},
		"c6gn.xlarge":       {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"c6i.12xlarge":      {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"c6i.16xlarge":      {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"c6i.24xlarge":      {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"c6i.2xlarge":       {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"c6i.32xlarge":      {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"c6i.4xlarge":       {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"c6i.8xlarge":       {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"c6i.large":         {Adapters: 3, IPv4: 10, IPv6: 10, HypervisorType: "nitro"},
		"c6i.metal":         {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: ""},
		"c6i.xlarge":        {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"cc2.8xlarge":       {Adapters: 8, IPv4: 30, IPv6: 0, HypervisorType: "xen"},
		"d2.2xlarge":        {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "xen"},
		"d2.4xlarge":        {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "xen"},
		"d2.8xlarge":        {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "xen"},
		"d2.xlarge":         {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "xen"},
		"d3.2xlarge":        {Adapters: 4, IPv4: 5, IPv6: 5, HypervisorType: "nitro"},
		"d3.4xlarge":        {Adapters: 4, IPv4: 10, IPv6: 10, HypervisorType: "nitro"},
		"d3.8xlarge":        {Adapters: 3, IPv4: 20, IPv6: 20, HypervisorType: "nitro"},
		"d3.xlarge":         {Adapters: 4, IPv4: 3, IPv6: 3, HypervisorType: "nitro"},
		"d3en.12xlarge":     {Adapters: 3, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"d3en.2xlarge":      {Adapters: 4, IPv4: 5, IPv6: 5, HypervisorType: "nitro"},
		"d3en.4xlarge":      {Adapters: 4, IPv4: 10, IPv6: 10, HypervisorType: "nitro"},
		"d3en.6xlarge":      {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"d3en.8xlarge":      {Adapters: 4, IPv4: 20, IPv6: 20, HypervisorType: "nitro"},
		"d3en.xlarge":       {Adapters: 4, IPv4: 3, IPv6: 3, HypervisorType: "nitro"},
		"dl1.24xlarge":      {Adapters: 60, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"f1.16xlarge":       {Adapters: 8, IPv4: 50, IPv6: 50, HypervisorType: "xen"},
		"f1.2xlarge":        {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "xen"},
		"f1.4xlarge":        {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "xen"},
		"g2.2xlarge":        {Adapters: 4, IPv4: 15, IPv6: 0, HypervisorType: "xen"},
		"g2.8xlarge":        {Adapters: 8, IPv4: 30, IPv6: 0, HypervisorType: "xen"},
		"g3.16xlarge":       {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "xen"},
		"g3.4xlarge":        {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "xen"},
		"g3.8xlarge":        {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "xen"},
		"g3s.xlarge":        {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "xen"},
		"g4ad.16xlarge":     {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"g4ad.2xlarge":      {Adapters: 2, IPv4: 4, IPv6: 4, HypervisorType: "nitro"},
		"g4ad.4xlarge":      {Adapters: 3, IPv4: 10, IPv6: 10, HypervisorType: "nitro"},
		"g4ad.8xlarge":      {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"g4ad.xlarge":       {Adapters: 2, IPv4: 4, IPv6: 4, HypervisorType: "nitro"},
		"g4dn.12xlarge":     {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"g4dn.16xlarge":     {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"g4dn.2xlarge":      {Adapters: 3, IPv4: 10, IPv6: 10, HypervisorType: "nitro"},
		"g4dn.4xlarge":      {Adapters: 3, IPv4: 10, IPv6: 10, HypervisorType: "nitro"},
		"g4dn.8xlarge":      {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"g4dn.metal":        {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: ""},
		"g4dn.xlarge":       {Adapters: 3, IPv4: 10, IPv6: 10, HypervisorType: "nitro"},
		"g5.12xlarge":       {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"g5.16xlarge":       {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"g5.24xlarge":       {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"g5.2xlarge":        {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"g5.48xlarge":       {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"g5.4xlarge":        {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"g5.8xlarge":        {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"g5.xlarge":         {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"g5g.16xlarge":      {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"g5g.2xlarge":       {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"g5g.4xlarge":       {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"g5g.8xlarge":       {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"g5g.metal":         {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: ""},
		"g5g.xlarge":        {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"h1.16xlarge":       {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "xen"},
		"h1.2xlarge":        {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "xen"},
		"h1.4xlarge":        {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "xen"},
		"h1.8xlarge":        {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "xen"},
		"i2.2xlarge":        {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "xen"},
		"i2.4xlarge":        {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "xen"},
		"i2.8xlarge":        {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "xen"},
		"i2.xlarge":         {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "xen"},
		"i3.16xlarge":       {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "xen"},
		"i3.2xlarge":        {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "xen"},
		"i3.4xlarge":        {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "xen"},
		"i3.8xlarge":        {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "xen"},
		"i3.large":          {Adapters: 3, IPv4: 10, IPv6: 10, HypervisorType: "xen"},
		"i3.metal":          {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: ""},
		"i3.xlarge":         {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "xen"},
		"i3en.12xlarge":     {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"i3en.24xlarge":     {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"i3en.2xlarge":      {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"i3en.3xlarge":      {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"i3en.6xlarge":      {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"i3en.large":        {Adapters: 3, IPv4: 10, IPv6: 10, HypervisorType: "nitro"},
		"i3en.metal":        {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: ""},
		"i3en.xlarge":       {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"im4gn.16xlarge":    {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"im4gn.2xlarge":     {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"im4gn.4xlarge":     {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"im4gn.8xlarge":     {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"im4gn.large":       {Adapters: 3, IPv4: 10, IPv6: 10, HypervisorType: "nitro"},
		"im4gn.xlarge":      {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"inf1.24xlarge":     {Adapters: 11, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"inf1.2xlarge":      {Adapters: 4, IPv4: 10, IPv6: 10, HypervisorType: "nitro"},
		"inf1.6xlarge":      {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"inf1.xlarge":       {Adapters: 4, IPv4: 10, IPv6: 10, HypervisorType: "nitro"},
		"is4gen.2xlarge":    {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"is4gen.4xlarge":    {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"is4gen.8xlarge":    {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"is4gen.large":      {Adapters: 3, IPv4: 10, IPv6: 10, HypervisorType: "nitro"},
		"is4gen.medium":     {Adapters: 2, IPv4: 4, IPv6: 4, HypervisorType: "nitro"},
		"is4gen.xlarge":     {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"m1.large":          {Adapters: 3, IPv4: 10, IPv6: 0, HypervisorType: "xen"},
		"m1.medium":         {Adapters: 2, IPv4: 6, IPv6: 0, HypervisorType: "xen"},
		"m1.small":          {Adapters: 2, IPv4: 4, IPv6: 0, HypervisorType: "xen"},
		"m1.xlarge":         {Adapters: 4, IPv4: 15, IPv6: 0, HypervisorType: "xen"},
		"m2.2xlarge":        {Adapters: 4, IPv4: 30, IPv6: 0, HypervisorType: "xen"},
		"m2.4xlarge":        {Adapters: 8, IPv4: 30, IPv6: 0, HypervisorType: "xen"},
		"m2.xlarge":         {Adapters: 4, IPv4: 15, IPv6: 0, HypervisorType: "xen"},
		"m3.2xlarge":        {Adapters: 4, IPv4: 30, IPv6: 0, HypervisorType: "xen"},
		"m3.large":          {Adapters: 3, IPv4: 10, IPv6: 0, HypervisorType: "xen"},
		"m3.medium":         {Adapters: 2, IPv4: 6, IPv6: 0, HypervisorType: "xen"},
		"m3.xlarge":         {Adapters: 4, IPv4: 15, IPv6: 0, HypervisorType: "xen"},
		"m4.10xlarge":       {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "xen"},
		"m4.16xlarge":       {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "xen"},
		"m4.2xlarge":        {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "xen"},
		"m4.4xlarge":        {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "xen"},
		"m4.large":          {Adapters: 2, IPv4: 10, IPv6: 10, HypervisorType: "xen"},
		"m4.xlarge":         {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "xen"},
		"m5.12xlarge":       {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"m5.16xlarge":       {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"m5.24xlarge":       {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"m5.2xlarge":        {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"m5.4xlarge":        {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"m5.8xlarge":        {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"m5.large":          {Adapters: 3, IPv4: 10, IPv6: 10, HypervisorType: "nitro"},
		"m5.metal":          {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: ""},
		"m5.xlarge":         {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"m5a.12xlarge":      {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"m5a.16xlarge":      {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"m5a.24xlarge":      {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"m5a.2xlarge":       {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"m5a.4xlarge":       {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"m5a.8xlarge":       {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"m5a.large":         {Adapters: 3, IPv4: 10, IPv6: 10, HypervisorType: "nitro"},
		"m5a.xlarge":        {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"m5ad.12xlarge":     {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"m5ad.16xlarge":     {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"m5ad.24xlarge":     {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"m5ad.2xlarge":      {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"m5ad.4xlarge":      {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"m5ad.8xlarge":      {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"m5ad.large":        {Adapters: 3, IPv4: 10, IPv6: 10, HypervisorType: "nitro"},
		"m5ad.xlarge":       {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"m5d.12xlarge":      {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"m5d.16xlarge":      {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"m5d.24xlarge":      {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"m5d.2xlarge":       {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"m5d.4xlarge":       {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"m5d.8xlarge":       {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"m5d.large":         {Adapters: 3, IPv4: 10, IPv6: 10, HypervisorType: "nitro"},
		"m5d.metal":         {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: ""},
		"m5d.xlarge":        {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"m5dn.12xlarge":     {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"m5dn.16xlarge":     {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"m5dn.24xlarge":     {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"m5dn.2xlarge":      {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"m5dn.4xlarge":      {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"m5dn.8xlarge":      {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"m5dn.large":        {Adapters: 3, IPv4: 10, IPv6: 10, HypervisorType: "nitro"},
		"m5dn.metal":        {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: ""},
		"m5dn.xlarge":       {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"m5n.12xlarge":      {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"m5n.16xlarge":      {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"m5n.24xlarge":      {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"m5n.2xlarge":       {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"m5n.4xlarge":       {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"m5n.8xlarge":       {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"m5n.large":         {Adapters: 3, IPv4: 10, IPv6: 10, HypervisorType: "nitro"},
		"m5n.metal":         {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: ""},
		"m5n.xlarge":        {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"m5zn.12xlarge":     {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"m5zn.2xlarge":      {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"m5zn.3xlarge":      {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"m5zn.6xlarge":      {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"m5zn.large":        {Adapters: 3, IPv4: 10, IPv6: 10, HypervisorType: "nitro"},
		"m5zn.metal":        {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: ""},
		"m5zn.xlarge":       {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"m6a.12xlarge":      {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"m6a.16xlarge":      {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"m6a.24xlarge":      {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"m6a.2xlarge":       {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"m6a.32xlarge":      {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"m6a.48xlarge":      {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"m6a.4xlarge":       {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"m6a.8xlarge":       {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"m6a.large":         {Adapters: 3, IPv4: 10, IPv6: 10, HypervisorType: "nitro"},
		"m6a.xlarge":        {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"m6g.12xlarge":      {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"m6g.16xlarge":      {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"m6g.2xlarge":       {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"m6g.4xlarge":       {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"m6g.8xlarge":       {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"m6g.large":         {Adapters: 3, IPv4: 10, IPv6: 10, HypervisorType: "nitro"},
		"m6g.medium":        {Adapters: 2, IPv4: 4, IPv6: 4, HypervisorType: "nitro"},
		"m6g.metal":         {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: ""},
		"m6g.xlarge":        {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"m6gd.12xlarge":     {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"m6gd.16xlarge":     {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"m6gd.2xlarge":      {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"m6gd.4xlarge":      {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"m6gd.8xlarge":      {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"m6gd.large":        {Adapters: 3, IPv4: 10, IPv6: 10, HypervisorType: "nitro"},
		"m6gd.medium":       {Adapters: 2, IPv4: 4, IPv6: 4, HypervisorType: "nitro"},
		"m6gd.metal":        {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: ""},
		"m6gd.xlarge":       {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"m6i.12xlarge":      {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"m6i.16xlarge":      {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"m6i.24xlarge":      {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"m6i.2xlarge":       {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"m6i.32xlarge":      {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"m6i.4xlarge":       {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"m6i.8xlarge":       {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"m6i.large":         {Adapters: 3, IPv4: 10, IPv6: 10, HypervisorType: "nitro"},
		"m6i.metal":         {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: ""},
		"m6i.xlarge":        {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"mac1.metal":        {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: ""},
		"p2.16xlarge":       {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "xen"},
		"p2.8xlarge":        {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "xen"},
		"p2.xlarge":         {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "xen"},
		"p3.16xlarge":       {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "xen"},
		"p3.2xlarge":        {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "xen"},
		"p3.8xlarge":        {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "xen"},
		"p3dn.24xlarge":     {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"p4d.24xlarge":      {Adapters: 60, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"r3.2xlarge":        {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "xen"},
		"r3.4xlarge":        {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "xen"},
		"r3.8xlarge":        {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "xen"},
		"r3.large":          {Adapters: 3, IPv4: 10, IPv6: 10, HypervisorType: "xen"},
		"r3.xlarge":         {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "xen"},
		"r4.16xlarge":       {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "xen"},
		"r4.2xlarge":        {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "xen"},
		"r4.4xlarge":        {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "xen"},
		"r4.8xlarge":        {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "xen"},
		"r4.large":          {Adapters: 3, IPv4: 10, IPv6: 10, HypervisorType: "xen"},
		"r4.xlarge":         {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "xen"},
		"r5.12xlarge":       {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"r5.16xlarge":       {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"r5.24xlarge":       {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"r5.2xlarge":        {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"r5.4xlarge":        {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"r5.8xlarge":        {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"r5.large":          {Adapters: 3, IPv4: 10, IPv6: 10, HypervisorType: "nitro"},
		"r5.metal":          {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: ""},
		"r5.xlarge":         {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"r5a.12xlarge":      {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"r5a.16xlarge":      {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"r5a.24xlarge":      {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"r5a.2xlarge":       {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"r5a.4xlarge":       {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"r5a.8xlarge":       {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"r5a.large":         {Adapters: 3, IPv4: 10, IPv6: 10, HypervisorType: "nitro"},
		"r5a.xlarge":        {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"r5ad.12xlarge":     {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"r5ad.16xlarge":     {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"r5ad.24xlarge":     {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"r5ad.2xlarge":      {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"r5ad.4xlarge":      {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"r5ad.8xlarge":      {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"r5ad.large":        {Adapters: 3, IPv4: 10, IPv6: 10, HypervisorType: "nitro"},
		"r5ad.xlarge":       {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"r5b.12xlarge":      {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"r5b.16xlarge":      {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"r5b.24xlarge":      {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"r5b.2xlarge":       {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"r5b.4xlarge":       {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"r5b.8xlarge":       {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"r5b.large":         {Adapters: 3, IPv4: 10, IPv6: 10, HypervisorType: "nitro"},
		"r5b.metal":         {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: ""},
		"r5b.xlarge":        {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"r5d.12xlarge":      {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"r5d.16xlarge":      {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"r5d.24xlarge":      {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"r5d.2xlarge":       {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"r5d.4xlarge":       {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"r5d.8xlarge":       {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"r5d.large":         {Adapters: 3, IPv4: 10, IPv6: 10, HypervisorType: "nitro"},
		"r5d.metal":         {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: ""},
		"r5d.xlarge":        {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"r5dn.12xlarge":     {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"r5dn.16xlarge":     {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"r5dn.24xlarge":     {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"r5dn.2xlarge":      {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"r5dn.4xlarge":      {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"r5dn.8xlarge":      {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"r5dn.large":        {Adapters: 3, IPv4: 10, IPv6: 10, HypervisorType: "nitro"},
		"r5dn.metal":        {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: ""},
		"r5dn.xlarge":       {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"r5n.12xlarge":      {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"r5n.16xlarge":      {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"r5n.24xlarge":      {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"r5n.2xlarge":       {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"r5n.4xlarge":       {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"r5n.8xlarge":       {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"r5n.large":         {Adapters: 3, IPv4: 10, IPv6: 10, HypervisorType: "nitro"},
		"r5n.metal":         {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: ""},
		"r5n.xlarge":        {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"r6g.12xlarge":      {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"r6g.16xlarge":      {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"r6g.2xlarge":       {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"r6g.4xlarge":       {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"r6g.8xlarge":       {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"r6g.large":         {Adapters: 3, IPv4: 10, IPv6: 10, HypervisorType: "nitro"},
		"r6g.medium":        {Adapters: 2, IPv4: 4, IPv6: 4, HypervisorType: "nitro"},
		"r6g.metal":         {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: ""},
		"r6g.xlarge":        {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"r6gd.12xlarge":     {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"r6gd.16xlarge":     {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"r6gd.2xlarge":      {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"r6gd.4xlarge":      {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"r6gd.8xlarge":      {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"r6gd.large":        {Adapters: 3, IPv4: 10, IPv6: 10, HypervisorType: "nitro"},
		"r6gd.medium":       {Adapters: 2, IPv4: 4, IPv6: 4, HypervisorType: "nitro"},
		"r6gd.metal":        {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: ""},
		"r6gd.xlarge":       {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"r6i.12xlarge":      {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"r6i.16xlarge":      {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"r6i.24xlarge":      {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"r6i.2xlarge":       {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"r6i.32xlarge":      {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"r6i.4xlarge":       {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"r6i.8xlarge":       {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"r6i.large":         {Adapters: 3, IPv4: 10, IPv6: 10, HypervisorType: "nitro"},
		"r6i.metal":         {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: ""},
		"r6i.xlarge":        {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"t1.micro":          {Adapters: 2, IPv4: 2, IPv6: 0, HypervisorType: "xen"},
		"t2.2xlarge":        {Adapters: 3, IPv4: 15, IPv6: 15, HypervisorType: "xen"},
		"t2.large":          {Adapters: 3, IPv4: 12, IPv6: 12, HypervisorType: "xen"},
		"t2.medium":         {Adapters: 3, IPv4: 6, IPv6: 6, HypervisorType: "xen"},
		"t2.micro":          {Adapters: 2, IPv4: 2, IPv6: 2, HypervisorType: "xen"},
		"t2.nano":           {Adapters: 2, IPv4: 2, IPv6: 2, HypervisorType: "xen"},
		"t2.small":          {Adapters: 3, IPv4: 4, IPv6: 4, HypervisorType: "xen"},
		"t2.xlarge":         {Adapters: 3, IPv4: 15, IPv6: 15, HypervisorType: "xen"},
		"t3.2xlarge":        {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"t3.large":          {Adapters: 3, IPv4: 12, IPv6: 12, HypervisorType: "nitro"},
		"t3.medium":         {Adapters: 3, IPv4: 6, IPv6: 6, HypervisorType: "nitro"},
		"t3.micro":          {Adapters: 2, IPv4: 2, IPv6: 2, HypervisorType: "nitro"},
		"t3.nano":           {Adapters: 2, IPv4: 2, IPv6: 2, HypervisorType: "nitro"},
		"t3.small":          {Adapters: 3, IPv4: 4, IPv6: 4, HypervisorType: "nitro"},
		"t3.xlarge":         {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"t3a.2xlarge":       {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"t3a.large":         {Adapters: 3, IPv4: 12, IPv6: 12, HypervisorType: "nitro"},
		"t3a.medium":        {Adapters: 3, IPv4: 6, IPv6: 6, HypervisorType: "nitro"},
		"t3a.micro":         {Adapters: 2, IPv4: 2, IPv6: 2, HypervisorType: "nitro"},
		"t3a.nano":          {Adapters: 2, IPv4: 2, IPv6: 2, HypervisorType: "nitro"},
		"t3a.small":         {Adapters: 2, IPv4: 4, IPv6: 4, HypervisorType: "nitro"},
		"t3a.xlarge":        {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"t4g.2xlarge":       {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"t4g.large":         {Adapters: 3, IPv4: 12, IPv6: 12, HypervisorType: "nitro"},
		"t4g.medium":        {Adapters: 3, IPv4: 6, IPv6: 6, HypervisorType: "nitro"},
		"t4g.micro":         {Adapters: 2, IPv4: 2, IPv6: 2, HypervisorType: "nitro"},
		"t4g.nano":          {Adapters: 2, IPv4: 2, IPv6: 2, HypervisorType: "nitro"},
		"t4g.small":         {Adapters: 3, IPv4: 4, IPv6: 4, HypervisorType: "nitro"},
		"t4g.xlarge":        {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"u-12tb1.112xlarge": {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"u-3tb1.56xlarge":   {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"u-6tb1.112xlarge":  {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"u-6tb1.56xlarge":   {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"u-9tb1.112xlarge":  {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"vt1.24xlarge":      {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"vt1.3xlarge":       {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"vt1.6xlarge":       {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"x1.16xlarge":       {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "xen"},
		"x1.32xlarge":       {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "xen"},
		"x1e.16xlarge":      {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "xen"},
		"x1e.2xlarge":       {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "xen"},
		"x1e.32xlarge":      {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "xen"},
		"x1e.4xlarge":       {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "xen"},
		"x1e.8xlarge":       {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "xen"},
		"x1e.xlarge":        {Adapters: 3, IPv4: 10, IPv6: 10, HypervisorType: "xen"},
		"x2gd.12xlarge":     {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"x2gd.16xlarge":     {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"x2gd.2xlarge":      {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"x2gd.4xlarge":      {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"x2gd.8xlarge":      {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"x2gd.large":        {Adapters: 3, IPv4: 10, IPv6: 10, HypervisorType: "nitro"},
		"x2gd.medium":       {Adapters: 2, IPv4: 4, IPv6: 4, HypervisorType: "nitro"},
		"x2gd.metal":        {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: ""},
		"x2gd.xlarge":       {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"x2iezn.12xlarge":   {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"x2iezn.2xlarge":    {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"x2iezn.4xlarge":    {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"x2iezn.6xlarge":    {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"x2iezn.8xlarge":    {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"x2iezn.metal":      {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: ""},
		"z1d.12xlarge":      {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: "nitro"},
		"z1d.2xlarge":       {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
		"z1d.3xlarge":       {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"z1d.6xlarge":       {Adapters: 8, IPv4: 30, IPv6: 30, HypervisorType: "nitro"},
		"z1d.large":         {Adapters: 3, IPv4: 10, IPv6: 10, HypervisorType: "nitro"},
		"z1d.metal":         {Adapters: 15, IPv4: 50, IPv6: 50, HypervisorType: ""},
		"z1d.xlarge":        {Adapters: 4, IPv4: 15, IPv6: 15, HypervisorType: "nitro"},
	}
}

// Get returns the instance limits of a particular instance type.
func Get(instanceType string) (limit ipamTypes.Limits, ok bool) {
	limitsOnce.Do(populateStaticENILimits)

	limits.RLock()
	limit, ok = limits.m[instanceType]
	limits.RUnlock()
	return
}

// UpdateFromUserDefinedMappings updates limits from the given map.
func UpdateFromUserDefinedMappings(m map[string]string) (err error) {
	limitsOnce.Do(populateStaticENILimits)

	limits.Lock()
	defer limits.Unlock()

	for instanceType, limitString := range m {
		limit, err := parseLimitString(limitString)
		if err != nil {
			return err
		}
		// Add or overwrite limits
		limits.m[instanceType] = limit
	}
	return nil
}

// UpdateFromEC2API updates limits from the EC2 API via calling
// https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeInstanceTypes.html.
func UpdateFromEC2API(ctx context.Context, ec2Client *ec2shim.Client) error {
	instanceTypeInfos, err := ec2Client.GetInstanceTypes(ctx)
	if err != nil {
		return err
	}

	limitsOnce.Do(populateStaticENILimits)

	limits.Lock()
	defer limits.Unlock()

	for _, instanceTypeInfo := range instanceTypeInfos {
		instanceType := string(instanceTypeInfo.InstanceType)
		adapterLimit := aws.ToInt32(instanceTypeInfo.NetworkInfo.MaximumNetworkInterfaces)
		ipv4PerAdapter := aws.ToInt32(instanceTypeInfo.NetworkInfo.Ipv4AddressesPerInterface)
		ipv6PerAdapter := aws.ToInt32(instanceTypeInfo.NetworkInfo.Ipv6AddressesPerInterface)
		hypervisorType := instanceTypeInfo.Hypervisor

		limits.m[instanceType] = ipamTypes.Limits{
			Adapters:       int(adapterLimit),
			IPv4:           int(ipv4PerAdapter),
			IPv6:           int(ipv6PerAdapter),
			HypervisorType: string(hypervisorType),
		}
	}

	return nil
}

// parseLimitString returns the Limits struct parsed from config string.
func parseLimitString(limitString string) (limit ipamTypes.Limits, err error) {
	intSlice := make([]int, 3)
	stringSlice := strings.Split(strings.ReplaceAll(limitString, " ", ""), ",")
	if len(stringSlice) != 3 {
		return limit, fmt.Errorf("invalid limit value")
	}
	for i, s := range stringSlice {
		intLimit, err := strconv.Atoi(s)
		if err != nil {
			return limit, err
		}
		intSlice[i] = intLimit
	}
	return ipamTypes.Limits{Adapters: intSlice[0], IPv4: intSlice[1], IPv6: intSlice[2]}, nil
}
