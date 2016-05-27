package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
)

const (
	namespace = "traefik"
)

type traefikHealth struct {
	//Pid                    int            `json:"pid"`
	//UpTime                 string         `json:"uptime"`
	UpTimeSec float64 `json:"uptime_sec"`
	//Time                   string         `json:"time"`
	//TimeUnix               int64          `json:"unixtime"`
	StatusCodeCount      map[string]float64 `json:"status_code_count"`
	TotalStatusCodeCount map[string]float64 `json:"total_status_code_count"`
	//Count                  float64            `json:"count"`
	//TotalCount             float64            `json:"total_count"`
	//TotalResponseTime      string         `json:"total_response_time"`
	TotalResponseTimeSec float64 `json:"total_response_time_sec"`
	//AverageResponseTime    string         `json:"average_response_time"`
	AverageResponseTimeSec float64 `json:"average_response_time_sec"`
}

func newCounter(metricName string, docString string, constLabels prometheus.Labels) prometheus.Counter {
	return prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace:   namespace,
			Name:        metricName,
			Help:        docString,
			ConstLabels: constLabels,
		},
	)
}

func newGauge(metricName string, docString string, constLabels prometheus.Labels) prometheus.Gauge {
	return prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace:   namespace,
			Name:        metricName,
			Help:        docString,
			ConstLabels: constLabels,
		},
	)
}

var (
	traefik_up                         = newGauge("up", "Is traefik up ?", nil)
	metric_uptime                      = newGauge("uptime", "Current Traefik uptime", nil)
	metric_request_response_time_total = newGauge("request_response_time_total", "Total response time of Traefik requests", nil)
	metric_request_response_time_avg   = newGauge("request_response_time_avg", "Average response time of Traefik requests", nil)

	// Labeled metrics
	// Set at runtime
	metric_request_status_count_current = map[string]prometheus.Gauge{} // newGauge("request_count_current", "Number of request Traefik is handling", nil)
	metric_request_status_count_total   = map[string]prometheus.Gauge{} // newGauge("request_count_total", "Number of request handled by Traefik", nil)
)

func init() {
	traefik_up.Set(0)
}

// Exporter collects Traefik stats from the given hostname and exports them using
// the prometheus metrics package.
type Exporter struct {
	mutex  sync.RWMutex
	URI    string
	client *http.Client
}

// NewExporter returns an initialized Exporter.
func NewExporter(uri string, timeout time.Duration) *Exporter {
	// Set up our Traefik client connection.
	return &Exporter{
		URI: uri,
		client: &http.Client{
			Transport: &http.Transport{
				Dial: func(netw, addr string) (net.Conn, error) {
					c, err := net.DialTimeout(netw, addr, timeout)
					if err != nil {
						return nil, err
					}
					if err := c.SetDeadline(time.Now().Add(timeout)); err != nil {
						return nil, err
					}
					return c, nil
				},
			},
		},
	}
}

// Describe describes all the metrics ever exported by the Traefik exporter. It
// implements prometheus.Collector.
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- metric_uptime.Desc()
	ch <- traefik_up.Desc()
	ch <- metric_request_response_time_total.Desc()
	ch <- metric_request_response_time_avg.Desc()

	for _, metric := range metric_request_status_count_current {
		ch <- metric.Desc()
	}
	for _, metric := range metric_request_status_count_total {
		ch <- metric.Desc()
	}
}

// Collect fetches the stats from configured Traefik location and delivers them
// as Prometheus metrics. It implements prometheus.Collector.
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	e.mutex.Lock() // To protect metrics from concurrent collects.
	defer e.mutex.Unlock()

	if err := e.scrape(); err != nil {
		log.Error(err)
		traefik_up.Set(0)
		ch <- traefik_up
		return
	}

	ch <- traefik_up
	ch <- metric_uptime
	ch <- metric_request_response_time_total
	ch <- metric_request_response_time_avg

	for _, metric := range metric_request_status_count_current {
		ch <- metric
	}
	for _, metric := range metric_request_status_count_total {
		ch <- metric
	}
}

func (e *Exporter) scrape() error {
	resp, err := e.client.Get(e.URI)
	if err != nil {
		return errors.New(fmt.Sprintf("Can't scrape Traefik: %v", err))
	}
	defer resp.Body.Close()
	if !(resp.StatusCode >= 200 && resp.StatusCode < 300) {
		return errors.New(fmt.Sprintf("Can't scrape Traefik: status %d", resp.StatusCode))
	}

	var data traefikHealth
	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(&data); err != nil {
		return errors.New(fmt.Sprintf("Can't scrape Traefik: json.Unmarshal %v", err))
	}

	traefik_up.Set(1)
	metric_uptime.Set(data.UpTimeSec)
	metric_request_response_time_total.Set(data.TotalResponseTimeSec)
	metric_request_response_time_avg.Set(data.AverageResponseTimeSec)

	// Current request count, labeled by statusCode
	// Must be reset for missing status code metrics in data
	for _, metric := range metric_request_status_count_current {
		metric.Set(0)
	}
	for statusCode, nbr := range data.StatusCodeCount {
		if _, ok := metric_request_status_count_current[statusCode]; ok == false {
			metric_request_status_count_current[statusCode] = newGauge("request_count_current", "Number of request handled by Traefik", prometheus.Labels{"statusCode": statusCode})
		}
		metric_request_status_count_current[statusCode].Set(nbr)
	}

	// Total request count, labeled by statusCode
	for statusCode, nbr := range data.TotalStatusCodeCount {
		if _, ok := metric_request_status_count_total[statusCode]; ok == false {
			metric_request_status_count_total[statusCode] = newGauge("request_count_total", "Number of request handled by Traefik", prometheus.Labels{"statusCode": statusCode})
		}
		metric_request_status_count_total[statusCode].Set(nbr)
	}

	return nil
}

func init() {
	prometheus.MustRegister(version.NewCollector("traefik_exporter"))
}

func main() {
	var (
		showVersion   = flag.Bool("version", false, "Print version information.")
		listenAddress = flag.String("web.listen-address", ":9000", "Address to listen on for web interface and telemetry.")
		metricsPath   = flag.String("web.telemetry-path", "/metrics", "Path under which to expose metrics.")
		traefikAddr   = flag.String("traefik.address", "http://localhost:8080/health", "HTTP API address of a Traefik or agent.")
		timeout       = flag.Duration("timeout", 5, "Timeout for trying to get stats from Traefik.")
	)
	flag.Parse()

	if *showVersion {
		fmt.Fprintln(os.Stdout, version.Print("traefik_exporter"))
		os.Exit(0)
	}

	log.Infoln("Starting traefik_exporter", version.Info())
	log.Infoln("Build context", version.BuildContext())

	exporter := NewExporter(*traefikAddr, *timeout*time.Second)
	prometheus.MustRegister(exporter)
	prometheus.Unregister(prometheus.NewGoCollector())
	prometheus.Unregister(prometheus.NewProcessCollector(os.Getpid(), ""))

	http.Handle(*metricsPath, prometheus.UninstrumentedHandler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
             <head><title>Traefik Exporter</title></head>
             <body>
             <h1>Traefik Exporter</h1>
             <p><a href='` + *metricsPath + `'>Metrics</a></p>
             </body>
             </html>`))
	})

	log.Infoln("Listening on", *listenAddress)
	log.Fatal(http.ListenAndServe(*listenAddress, nil))
}
