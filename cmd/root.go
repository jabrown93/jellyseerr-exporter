package cmd

import (
	"net/http"
	"strings"

	"github.com/opspotes/jellyseerr-exporter/collector"
	"github.com/opspotes/jellyseerr-exporter/internal/jellyseerr"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	logLevel            string
	jellyseerrAddress   string
	jellyseerrAPIKey    string
	jellyseerrAPILocale string
	fullData            bool
)

// instance to use
var jellyseerrClient *jellyseerr.Client

var RootCmd = &cobra.Command{
	Use:   "jellyseerr-exporter",
	Short: "Export request metrics from Jellyseerr",
	Long:  `Export request metrics from a Jellyseerr instance for Prometheus.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		setupLogger()
		logrus.WithFields(logrus.Fields{
			"command": cmd.Name(),
			"args":    args,
		}).Debugln("Running command")
	},
	PreRun: func(cmd *cobra.Command, args []string) {
		// Get values from viper
		logLevel = viper.GetString("log")
		jellyseerrAddress = viper.GetString("jellyseerr.address")
		jellyseerrAPIKey = viper.GetString("jellyseerr.apiKey")
		jellyseerrAPILocale = viper.GetString("jellyseerr.locale")
		fullData = viper.GetBool("fullData")

		// Validate required flags
		if jellyseerrAddress == "" {
			logrus.Fatalln("La configuration 'jellyseerr.address' est requise. Fournissez-la via le flag --jellyseerr.address ou la variable d'environnement JELLYSEERR_ADDRESS.")
		}
		if jellyseerrAPIKey == "" {
			logrus.Fatalln("La configuration 'jellyseerr.apiKey' est requise. Fournissez-la via le flag --jellyseerr.apiKey ou la variable d'environnement JELLYSEERR_APIKEY.")
		}

		setJellyseerr()
	},
	Run: func(cmd *cobra.Command, args []string) {
		prometheus.MustRegister(prometheus.NewBuildInfoCollector())
		prometheus.MustRegister(collector.NewRequestCollector(jellyseerrClient, fullData))
		prometheus.MustRegister(collector.NewUserCollector(jellyseerrClient))

		// Handle Metrics endpoint
		promHandler := promhttp.Handler()
		http.Handle("/metrics", promHandler)

		// Default exporter redirect message on /
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`<html>
		<head><title>Jellyseerr Exporter</title></head>
		<body>
		<h1>Jellyseerr Exporter</h1>
		<p><a href="/metrics">Metrics</a></p>
		</body>
		</html>`))
		})

		if err := http.ListenAndServe(":9850", nil); err != nil {
			logrus.WithField("err msg", err.Error()).Fatalln("HTTP server failed: exiting")
		}
	},
}

func setupLogger() {
	if level, err := logrus.ParseLevel(logLevel); err != nil {
		logrus.SetLevel(logrus.FatalLevel)
	} else {
		logrus.SetLevel(level)
	}
}

func setJellyseerr() {
	if o, err := jellyseerr.NewKeyAuth(jellyseerrAddress, jellyseerrAPILocale, jellyseerrAPIKey); err != nil {
		logrus.WithField("message", err.Error()).Fatalln("Could not connect to Jellyseerr")
	} else {
		jellyseerrClient = o
	}
}

func initConfig() {
	viper.AutomaticEnv()
	// Replace points by underscore when parsing env variables
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	viper.BindPFlag("log", RootCmd.PersistentFlags().Lookup("log"))
	viper.BindPFlag("jellyseerr.address", RootCmd.PersistentFlags().Lookup("jellyseerr.address"))
	viper.BindPFlag("jellyseerr.apiKey", RootCmd.PersistentFlags().Lookup("jellyseerr.apiKey"))
	viper.BindPFlag("jellyseerr.locale", RootCmd.PersistentFlags().Lookup("jellyseerr.locale"))
	viper.BindPFlag("fullData", RootCmd.PersistentFlags().Lookup("fullData"))
}

func init() {
	cobra.OnInitialize(initConfig)

	RootCmd.PersistentFlags().StringVar(&logLevel, "log", "fatal", "set the log level (fatal, error, info, debug, trace)")
	RootCmd.PersistentFlags().StringVar(&jellyseerrAddress, "jellyseerr.address", "", "Address at which Jellyseerr is hosted.")
	RootCmd.PersistentFlags().StringVar(&jellyseerrAPIKey, "jellyseerr.apiKey", "", "API key for admin access to the Jellyseerr instance.")
	RootCmd.PersistentFlags().StringVar(&jellyseerrAPILocale, "jellyseerr.locale", "en", "Locale of the Jellyseerr instance.")
	RootCmd.PersistentFlags().BoolVar(&fullData, "fullData", false, "Reduce scraping and cardinality on requests count metric.")
}
