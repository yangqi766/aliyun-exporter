package client

import (
	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/requests"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/rds"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/slb"
)

/* type Credential struct {
	ak     string
	sk     string
	region string
} */

/* func RetriveList(
	namespace string,
	credential config.Credential,
) (map[string]map[string]string, error) {
	c := &Credential{
		ak:     credential.AccessKey,
		sk:     credential.AccessKeySecret,
		region: credential.Region,
	}
	instances := make(map[string]map[string]string)
	if namespace == "acs_slb_dashboard" {
		instances, err := c.SlbResourceList()
		return instances, err
	}
	if namespace == "acs_rds_dashboard" {
		instances, err := c.RdsResourceList()
		return instances, err
	}
	return instances, nil
} */

func (c *MetricClient) RetriveSLbs() (map[string]map[string]string, error) {
	slbClient, err := slb.NewClientWithAccessKey(
		c.credentials.Region,
		c.credentials.AccessKey,
		c.credentials.AccessKeySecret,
	)
	if err != nil {
		return nil, err
	}
	req := slb.CreateDescribeLoadBalancersRequest()
	req.PageSize = requests.NewInteger(100)
	resp, err := slbClient.DescribeLoadBalancers(req)
	if err != nil {
		return nil, err
	}
	instances := make(map[string]map[string]string)
	for _, v := range resp.LoadBalancers.LoadBalancer {
		if v.LoadBalancerName != "" {
			instances[v.LoadBalancerId] = map[string]string{
				"loadbalancerName": v.LoadBalancerName,
			}
		} else {
			instances[v.LoadBalancerId] = map[string]string{
				"loadBalancerName": v.LoadBalancerId,
			}
		}
	}
	return instances, nil
}

func (c *MetricClient) RetriveRds() (map[string]map[string]string, error) {
	rdsClient, err := rds.NewClientWithAccessKey(
		c.credentials.Region,
		c.credentials.AccessKey,
		c.credentials.AccessKeySecret,
	)
	if err != nil {
		return nil, err
	}
	req := rds.CreateDescribeDBInstancesRequest()
	req.PageSize = requests.NewInteger(100)
	resp, err := rdsClient.DescribeDBInstances(req)
	if err != nil {
		return nil, err
	}
	instances := make(map[string]map[string]string)
	for _, v := range resp.Items.DBInstance {
		if v.DBInstanceDescription != "" {
			instances[v.DBInstanceId] = map[string]string{
				"DBInstanceDescription": v.DBInstanceDescription,
			}
		} else {
			instances[v.DBInstanceId] = map[string]string{
				"DBInstanceDescription": v.DBInstanceDescription,
			}
		}
	}
	return instances, nil
}
