package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"io/ioutil"
	"log"
	"net/http"
)

const taskStatSQL = "SELECT type,status,count(*) AS total FROM sys.tasks GROUP BY type,status"

var (
	addr     = flag.String("listen-address", ":8080", "The address to listen on for HTTP requests.")
	druidUri = flag.String("druid-uri", "http://BROKER:8082/druid/v2/sql/", "The URI to reach Druid's SQL API.")
)

type Task struct {
	Type   string
	Status string
	Total  int
}

type DruidTasksExporter struct {
	TasksStats *prometheus.Desc
}

func NewDruidTasksExporter() *DruidTasksExporter {
	return &DruidTasksExporter{
		TasksStats: prometheus.NewDesc(
			"dte_druid_tasks_total",
			"Total number of Druid tasks per type and status.",
			[]string{"type", "status"},
			prometheus.Labels{},
		)}
}

func (d *DruidTasksExporter) RetrieveStats() []Task {

	query, _ := json.Marshal(map[string]string{
		"query": taskStatSQL,
	})

	reqBody := bytes.NewBuffer(query)
	resp, err := http.Post(*druidUri, "application/json", reqBody)
	if err != nil {
		log.Fatalf("An Error occured while making the request: %v", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("An Error occured while reading the response: %v", err)
	}

	var tasks []Task
	// If some fields are missing, will crash later.
	err = json.Unmarshal(body, &tasks)
	if err != nil {
		log.Fatalf("An Error occured while unmarshalling %s: %v", body, err)
	}

	return tasks
}

func (c *DruidTasksExporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.TasksStats
}

func (d *DruidTasksExporter) Collect(ch chan<- prometheus.Metric) {
	tasks := d.RetrieveStats()
	for _, task := range tasks {
		ch <- prometheus.MustNewConstMetric(
			d.TasksStats,
			prometheus.GaugeValue,
			float64(task.Total),
			task.Type,
			task.Status,
		)
	}
}

func main() {
	flag.Parse()

	dte := NewDruidTasksExporter()
	reg := prometheus.NewPedanticRegistry()
	reg.MustRegister(dte)

	http.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	log.Printf("The server is listening on %s and scraping %s", *addr, *druidUri)
	log.Fatal(http.ListenAndServe(*addr, nil))
}
