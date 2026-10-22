package collector

import (
	"strconv"
	"time"

	"github.com/opspotes/jellyseerr-exporter/internal/jellyseerr"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

const (
	requestPageSize int = 50
)

type RequestCollector struct {
	client   *jellyseerr.Client
	fullData bool

	Count *prometheus.Desc
	up    *prometheus.Desc
}

type RequestMetricLabel struct {
	MediaType     string
	RequestStatus string
	MediaStatus   string
	UHD           string
	Company       string
	Genre         string
}

func NewRequestCollector(client *jellyseerr.Client, fullData bool) *RequestCollector {
	logrus.Traceln("defining request collector")
	specificNamespace := "requests"
	return &RequestCollector{
		client:   client,
		fullData: fullData,

		Count: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, specificNamespace, "count"),
			"Number of requests on Jellyseerr",
			[]string{"media_type", "is_4k", "request_status", "media_status", "genre", "company"},
			nil,
		),
		up: newUpDesc("requests"),
	}
}

func (rc *RequestCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- rc.Count
	ch <- rc.up
}

func (rc *RequestCollector) Collect(ch chan<- prometheus.Metric) {
	logrus.Debug("collecting request data...")
	start := time.Now()

	allRequests, ok := fetchAllRequests(rc.client)
	if !ok {
		ch <- prometheus.MustNewConstMetric(rc.up, prometheus.GaugeValue, 0)
		return
	}
	ch <- prometheus.MustNewConstMetric(rc.up, prometheus.GaugeValue, 1)

	counts := make(map[RequestMetricLabel]int)
	for _, request := range allRequests {
		genre := "not_collected"
		company := "not_collected"
		if rc.fullData {
			switch request.Media.MediaType {
			case jellyseerr.MediaTypeMovie:
				if details, err := request.GetMovieDetails(rc.client); err == nil {
					if len(details.Genres) > 0 {
						genre = details.Genres[0].Name
					}
					if len(details.ProductionCompanies) > 0 {
						company = details.ProductionCompanies[0].Name
					}
				}
			case jellyseerr.MediaTypeTV:
				if details, err := request.GetTVDetails(rc.client); err == nil {
					if len(details.Genres) > 0 {
						genre = details.Genres[0].Name
					}
					if len(details.Networks) > 0 {
						company = details.Networks[0].Name
					}
				}
			}
		}

		counts[RequestMetricLabel{
			RequestStatus: request.Status.ToString(),
			MediaStatus:   request.Media.Status.ToString(),
			MediaType:     string(request.Media.MediaType),
			UHD:           strconv.FormatBool(request.IsUHD),
			Genre:         genre,
			Company:       company,
		}]++
	}

	for labelSet, count := range counts {
		ch <- prometheus.MustNewConstMetric(
			rc.Count,
			prometheus.GaugeValue,
			float64(count),
			labelSet.MediaType, labelSet.UHD, labelSet.RequestStatus, labelSet.MediaStatus, labelSet.Genre, labelSet.Company,
		)
	}

	elapsed := time.Since(start)
	logrus.WithField("time_elapsed", elapsed).Debugln("request data collected")
}

func fetchAllRequests(client *jellyseerr.Client) ([]*jellyseerr.MediaRequest, bool) {
	var allRequests []*jellyseerr.MediaRequest
	page := 0
	for {
		logrus.WithField("page", page).Traceln("fetching request list from jellyseerr")
		requests, pageInfo, err := client.GetRequests(page, requestPageSize, jellyseerr.RequestFilterAll, jellyseerr.RequestSortAdded)
		if err != nil {
			logrus.WithField("page", page).Errorln("failed to get page of media requests from jellyseerr")
			return nil, false
		}
		allRequests = append(allRequests, requests...)
		page++
		if page >= pageInfo.Pages {
			break
		}
	}
	logrus.WithField("total_requests", len(allRequests)).Traceln("fetched all requests from jellyseerr")
	return allRequests, true
}
