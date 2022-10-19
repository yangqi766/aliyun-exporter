package client

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/IAOTW/aliyun-exporter/pkg/config"
	"github.com/IAOTW/aliyun-exporter/pkg/ratelimit"
	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/requests"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/cms"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
)

var ignores = map[string]struct{}{
	"timestamp": {},
	"Maximum":   {},
	"Minimum":   {},
	"Average":   {},
	"userId":    {},
}

// map for all avaliable namespaces
// todo: is there a way to add desc into yaml file?
var allNamespaces = map[string]string{
	"acs_ecs_dashboard":              "Elastic Compute Service (ECS)",
	"acs_containerservice_dashboard": "Container Service for Swarm",
	"acs_kubernetes":                 "Container Service for Kubernetes (ACK)",
	"acs_oss_dashboard":              "Object Storage Service (OSS)",
	"acs_slb_dashboard":              "Server Load Balancer (SLB)",
	"acs_vpc_eip":                    "Elastic IP addresses (EIPs)",
	"acs_nat_gateway":                "NAT Gateway",
	"acs_anycast_eip":                "Anycast Elastic IP address (EIP)",
	"acs_rds_dashboard":              "ApsaraDB RDS",
	"acs_mongodb":                    "ApsaraDB for MongoDB",
	"acs_memcache":                   "ApsaraDB for Memcache",
	"acs_kvstore":                    "ApsaraDB for Redis",
	"acs_hitsdb":                     "Time Series Database (TSDB)",
	"acs_clickhouse":                 "ClickHouse",
	"acs_cds":                        "ApsaraDB for Cassandra",
	"waf":                            "Web Application Firewall (WAF)",
	"acs_elasticsearch":              "Elasticsearch",
	"acs_mns_new":                    "queues of Message Service (MNS)",
	"acs_kafka":                      "Message Queue for Apache Kafka",
	"acs_amqp":                       "Alibaba Cloud Message Queue for AMQP instances",
}

// AllNamespaces return allNamespaces
func AllNamespaces() map[string]string {
	return allNamespaces
}

// allNamesOfNamespaces return all avaliable namespaces
func allNamesOfNamespaces() []string {
	ss := make([]string, 0, len(allNamespaces))
	for name := range allNamespaces {
		ss = append(ss, name)
	}
	return ss
}

// Datapoint datapoint
type Datapoint map[string]interface{}

// Get return value for measure
func (d Datapoint) Get(measure string) float64 {
	v, ok := d[measure]
	if !ok {
		return 0
	}
	return v.(float64)
}

// Labels return labels that not in ignores
func (d Datapoint) Labels(dimensions []string) []string {
	labels := make([]string, 0)
	maps := make(map[string]bool)
	for k := range d {
		if _, ok := ignores[k]; !ok {
			labels = append(labels, k)
			maps[k] = true
		}
	}
	for _, dim := range dimensions {
		if !maps[dim] {
			labels = append(labels, dim)
		}
	}
	return labels
}

// Values return values for lables
func (d Datapoint) Values(labels []string, retrives map[string]map[string]string) []string {
	var values []string
	for _, label := range labels {
		if v, ok := d[label]; ok {
			values = append(values, fmt.Sprintf("%s", v))
		} else {
			instanceId := d["instanceId"]
			values = append(values, retrives[instanceId.(string)][label])
		}
	}
	return values
}

// MetricClient wrap cms.client
type MetricClient struct {
	cloudID     string
	cms         *cms.Client
	logger      log.Logger
	credentials config.Credential
}

// NewMetricClient create metric Client
func NewMetricClient(
	cloudID string, credential config.Credential,
	logger log.Logger,
) (*MetricClient, error) {
	cmsClient, err := cms.NewClientWithAccessKey(
		credential.Region,
		credential.AccessKey,
		credential.AccessKeySecret,
	)
	if err != nil {
		return nil, err
	}
	// cmsClient.SetTransport(rt)
	if logger == nil {
		logger = log.NewNopLogger()
	}
	return &MetricClient{cloudID, cmsClient, logger, credential}, nil
}

func (c *MetricClient) SetTransport(rate int) {
	rt := ratelimit.New(rate)
	c.cms.SetTransport(rt)
}

// retrive get datapoints for metric
func (c *MetricClient) retrive(sub string, name string, period string) ([]Datapoint, error) {
	req := cms.CreateDescribeMetricLastRequest()
	req.ReadTimeout = 50 * time.Second
	req.Namespace = sub
	req.MetricName = name
	req.Period = period
	resp, err := c.cms.DescribeMetricLast(req)
	if err != nil {
		return nil, err
	}

	var datapoints []Datapoint
	if err = json.Unmarshal([]byte(resp.Datapoints), &datapoints); err != nil {
		// some execpected error
		level.Debug(c.logger).Log("content", resp.GetHttpContentString(), "error", err)
		return nil, err
	}

	return datapoints, nil
}

// Collect do collect metrics into channel
func (c *MetricClient) Collect(
	namespace string,
	sub string,
	m *config.Metric,
	ch chan<- prometheus.Metric,
) {
	if m.Name == "" {
		level.Warn(c.logger).Log("msg", "metric name must been set")
		return
	}

	datapoints, err := c.retrive(sub, m.Name, m.Period)
	if err != nil {
		level.Error(c.logger).
			Log("msg", "failed to retrive datapoints", "cloudID", c.cloudID, "namespace", sub, "name", m.String(), "error", err)
		return
	}

	var retrives map[string]map[string]string
	if sub == "acs_slb_dashboard" {
		retrives, _ = c.RetriveSLbs()
	}
	if sub == "acs_rds_dashboard" {
		retrives, _ = c.RetriveRds()
	}

	for _, dp := range datapoints {
		measure := dp.Get(m.Measure)

		labels := dp.Labels(m.Dimensions)
		fmt.Println(labels)
		values := dp.Values(labels, retrives)
		fmt.Println(values)

		ch <- prometheus.MustNewConstMetric(
			m.Desc(namespace, sub, labels...),
			prometheus.GaugeValue,
			measure,
			values...,
		)
	}
}

// DescribeMetricMetaList return metrics meta list
func (c *MetricClient) DescribeMetricMetaList(
	namespaces ...string,
) (map[string][]cms.Resource, error) {
	namespaces = filterNamespaces(namespaces...)
	m := make(map[string][]cms.Resource)
	for _, ns := range namespaces {
		req := cms.CreateDescribeMetricMetaListRequest()
		req.Namespace = ns
		req.PageSize = requests.NewInteger(100)
		resp, err := c.cms.DescribeMetricMetaList(req)
		if err != nil {
			return nil, err
		}
		level.Debug(c.logger).Log("content", resp.GetHttpContentString())
		m[ns] = resp.Resources.Resource
	}
	return m, nil
}

func filterNamespaces(namespaces ...string) []string {
	if len(namespaces) == 0 {
		return allNamesOfNamespaces()
	}
	filters := make([]string, 0)
	for _, ns := range namespaces {
		if ns == "all" {
			return allNamesOfNamespaces()
		}
		if _, ok := allNamespaces[ns]; ok {
			filters = append(filters, ns)
		}
	}
	return filters
}
