package collector

import (
	"context"
	"log"
	"math"
	"sync"
	"time"

	"github.com/mafredri/electrolux-ocp/ocpapi"
	"github.com/prometheus/client_golang/prometheus"
)

var labels = []string{
	// General appliance info.
	"pnc",
	"brand",
	// "market",
	"product_area",
	"device_type",
	// "project",
	"model",
	"variant",
	// "colour",

	// Appliance reported properties.
	"appliance_id",
	"name",
	"model_name",
	// "firmware_version",
	// "firmware_version_niu",
	// "firmware_version_mcu",
	// "tvoc_brand",
}

const namespace = "electrolux"

type Collector struct {
	client  *ocpapi.Client
	ctx     context.Context
	cancel  context.CancelFunc
	options Options

	mu             sync.Mutex
	applianceInfos map[string]ocpapi.ApplianceInfo

	airPurifierConnected   *prometheus.Desc
	airPurifierWorkmode    *prometheus.Desc
	airPurifierDoorOpen    *prometheus.Desc
	airPurifierUILight     *prometheus.Desc
	airPurifierSafetyLock  *prometheus.Desc
	airPurifierIonizer     *prometheus.Desc
	airPurifierFilterLife  *prometheus.Desc
	airPurifierFilterType  *prometheus.Desc
	airPurifierRSSI        *prometheus.Desc
	airPurifierFanspeed    *prometheus.Desc
	airPurifierFanspeedMax *prometheus.Desc
	airPurifierFanspeedRaw *prometheus.Desc
	airPurifierTemperature *prometheus.Desc
	airPurifierHumidity    *prometheus.Desc
	airPurifierPM1         *prometheus.Desc
	airPurifierPM25        *prometheus.Desc
	airPurifierPM10        *prometheus.Desc
	airPurifierCO2         *prometheus.Desc
	airPurifierTVOC        *prometheus.Desc
	airPurifierVOCDensity  *prometheus.Desc
}

type Options struct {
	MolecularWeight float64 // Molecular weight of gas, in g/mol. Used for TVOC ppb conversion to μg/m^3.
}

func NewCollector(client *ocpapi.Client, opts *Options) *Collector {
	if opts == nil {
		opts = &Options{}
	}
	if opts.MolecularWeight == 0 {
		opts.MolecularWeight = 30.026 // Formaldehyde (CH2O).
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &Collector{
		client:  client,
		ctx:     ctx,
		cancel:  cancel,
		options: *opts,

		applianceInfos: make(map[string]ocpapi.ApplianceInfo),

		airPurifierConnected:   prometheus.NewDesc(prometheus.BuildFQName(namespace, "appliance", "connected"), "Appliance is connected", labels, nil),
		airPurifierWorkmode:    prometheus.NewDesc(prometheus.BuildFQName(namespace, "appliance", "workmode"), "Work mode (PowerOff = 0, Manual = 1, Auto = 2, Quiet = 3)", labels, nil),
		airPurifierDoorOpen:    prometheus.NewDesc(prometheus.BuildFQName(namespace, "appliance", "door_open"), "Door is open", labels, nil),
		airPurifierUILight:     prometheus.NewDesc(prometheus.BuildFQName(namespace, "appliance", "ui_light"), "UI light enabled", labels, nil),
		airPurifierSafetyLock:  prometheus.NewDesc(prometheus.BuildFQName(namespace, "appliance", "safety_lock"), "Safety lock enabled", labels, nil),
		airPurifierIonizer:     prometheus.NewDesc(prometheus.BuildFQName(namespace, "appliance", "ionizer"), "Ionizer enabled", labels, nil),
		airPurifierFilterLife:  prometheus.NewDesc(prometheus.BuildFQName(namespace, "appliance", "filter_life"), "Filter life remaining", labels, nil),
		airPurifierFilterType:  prometheus.NewDesc(prometheus.BuildFQName(namespace, "appliance", "filter_type_id"), "Filter type as numeric ID", labels, nil),
		airPurifierRSSI:        prometheus.NewDesc(prometheus.BuildFQName(namespace, "appliance", "rssi"), "WiFi signal strength", labels, nil),
		airPurifierFanspeed:    prometheus.NewDesc(prometheus.BuildFQName(namespace, "appliance", "fanspeed"), "Fan speed", labels, nil),
		airPurifierFanspeedMax: prometheus.NewDesc(prometheus.BuildFQName(namespace, "appliance", "fanspeed_max"), "Maximum fan speed raw value", labels, nil),
		airPurifierFanspeedRaw: prometheus.NewDesc(prometheus.BuildFQName(namespace, "appliance", "fanspeed_raw"), "Fan speed (raw)", labels, nil),
		airPurifierTemperature: prometheus.NewDesc(prometheus.BuildFQName(namespace, "appliance", "temperature"), "Temperature in Celsius", labels, nil),
		airPurifierHumidity:    prometheus.NewDesc(prometheus.BuildFQName(namespace, "appliance", "humidity"), "Relative humidity", labels, nil),
		airPurifierPM1:         prometheus.NewDesc(prometheus.BuildFQName(namespace, "appliance", "pm1"), "PM1 in μg/m^3", labels, nil),
		airPurifierPM25:        prometheus.NewDesc(prometheus.BuildFQName(namespace, "appliance", "pm25"), "PM2.5 in μg/m^3", labels, nil),
		airPurifierPM10:        prometheus.NewDesc(prometheus.BuildFQName(namespace, "appliance", "pm10"), "PM10 in μg/m^3", labels, nil),
		airPurifierCO2:         prometheus.NewDesc(prometheus.BuildFQName(namespace, "appliance", "co2"), "CO2", labels, nil),
		airPurifierTVOC:        prometheus.NewDesc(prometheus.BuildFQName(namespace, "appliance", "tvoc_ppb"), "Total volatile organic compounds in ppb", labels, nil),
		airPurifierVOCDensity:  prometheus.NewDesc(prometheus.BuildFQName(namespace, "appliance", "voc_density"), "Volatile organic compound density in μg/m^3)", labels, nil),
	}
}

func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.airPurifierConnected
	ch <- c.airPurifierWorkmode
	ch <- c.airPurifierDoorOpen
	ch <- c.airPurifierUILight
	ch <- c.airPurifierSafetyLock
	ch <- c.airPurifierIonizer
	ch <- c.airPurifierFilterLife
	ch <- c.airPurifierFilterType
	ch <- c.airPurifierRSSI
	ch <- c.airPurifierFanspeed
	ch <- c.airPurifierFanspeedMax
	ch <- c.airPurifierFanspeedRaw
	ch <- c.airPurifierTemperature
	ch <- c.airPurifierHumidity
	ch <- c.airPurifierPM1
	ch <- c.airPurifierPM25
	ch <- c.airPurifierPM10
	ch <- c.airPurifierCO2
	ch <- c.airPurifierTVOC
	ch <- c.airPurifierVOCDensity
}

var signalStrengthMap = make(map[string]map[int]int)

func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	c.mu.Lock()
	defer c.mu.Unlock()

	log.Println("Collecting metrics...")

	ctx, cancel := context.WithTimeout(c.ctx, 30*time.Second)
	defer cancel()

	appliances, err := c.client.Appliances(ctx, true)
	if err != nil {
		log.Printf("Error fetching air purifiers: %v", err)
		return
	}

	var applianceIDs []string
	for _, appliance := range appliances {
		if _, ok := c.applianceInfos[appliance.ApplianceID.PNC()]; !ok {
			applianceIDs = append(applianceIDs, appliance.ApplianceID.String())
		}
	}

	if len(applianceIDs) > 0 {
		applianceInfo, err := c.client.AppliancesInfo(ctx, applianceIDs...)
		if err != nil {
			log.Printf("Error fetching appliance info: %v\n", err)
			return
		}

		for _, info := range applianceInfo {
			c.applianceInfos[info.PNC] = info
		}
	}

	for _, appliance := range appliances {
		info := c.applianceInfos[appliance.ApplianceID.PNC()]
		reported := appliance.Properties.Reported

		if info.DeviceType != "AIR_PURIFIER" {
			log.Printf("Skipping appliance %s with device type %s...\n", appliance.ApplianceID, info.DeviceType)
			continue
		}

		log.Printf("Collecting metrics for appliance %s (%s)...\n", appliance.ApplianceID, appliance.ApplianceData.ApplianceName)

		// TODO(mafredri): Define separate metric for appliance_info?

		labels := []string{
			info.PNC,
			info.Brand,
			// info.Market,
			info.ProductArea,
			info.DeviceType,
			// info.Project,
			info.Model,
			info.Variant,
			// info.Colour,

			appliance.ApplianceID.String(),
			appliance.ApplianceData.ApplianceName,
			appliance.ApplianceData.ModelName,
			// maybe(reported.FrmVerNIU), // Present on e.g. Pure A9, not on Pure 5000.
			// reported.VmNoNIU,
			// maybe(reported.VmNoMCU), // Present on e.g. Pure 500, not on Pure A9.
			// maybe(reported.TVOCBrand),
		}

		collectMetric := func(desc *prometheus.Desc, v float64) {
			ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, v, labels...)
		}
		maybeCollectIntMetric := func(desc *prometheus.Desc, v *int) {
			if v != nil {
				collectMetric(desc, float64(*v))
			}
		}
		maybeCollectBoolMetric := func(desc *prometheus.Desc, v *bool) {
			if v != nil {
				collectMetric(desc, float64(boolToFloat64(*v)))
			}
		}

		collectMetric(c.airPurifierConnected, boolToFloat64(appliance.ConnectionState == "Connected"))
		collectMetric(c.airPurifierWorkmode, workmode(reported.Workmode))
		maybeCollectBoolMetric(c.airPurifierDoorOpen, reported.DoorOpen)
		maybeCollectBoolMetric(c.airPurifierUILight, &reported.UILight)
		maybeCollectBoolMetric(c.airPurifierSafetyLock, &reported.SafetyLock)
		maybeCollectBoolMetric(c.airPurifierIonizer, reported.Ionizer)

		var filterLife *int
		switch {
		case reported.FilterLife1 != nil && reported.FilterLife != nil:
			if reported.Metadata.FilterLife1.LastUpdated.After(reported.Metadata.FilterLife.LastUpdated) {
				filterLife = reported.FilterLife1
			} else {
				filterLife = reported.FilterLife
			}
		case reported.FilterLife1 != nil:
			filterLife = reported.FilterLife1
		case reported.FilterLife != nil:
			filterLife = reported.FilterLife
		}
		if filterLife != nil {
			ratio := float64(*filterLife) / 100
			collectMetric(c.airPurifierFilterLife, ratio)
		}
		maybeCollectIntMetric(c.airPurifierFilterType, reported.FilterType)

		maybeCollectIntMetric(c.airPurifierRSSI, reported.RSSI)
		// collectMetric(c.airPurifierRSSI, signalStrengthToRSSI(reported.SignalStrength))

		if fanspeed, fanspeedMax, ok := fanspeed(appliance.ApplianceData.ModelName, reported.Fanspeed); ok {
			collectMetric(c.airPurifierFanspeed, round(fanspeed, 2))
			collectMetric(c.airPurifierFanspeedMax, fanspeedMax)
		}
		collectMetric(c.airPurifierFanspeedRaw, float64(reported.Fanspeed))

		if reported.Temp != nil {
			collectMetric(c.airPurifierTemperature, float64(*reported.Temp))
		}
		if reported.Humidity != nil {
			collectMetric(c.airPurifierHumidity, float64(*reported.Humidity)/100)
		}

		if reported.PM1 != nil {
			collectMetric(c.airPurifierPM1, float64(*reported.PM1))
		}
		switch {
		case reported.PM25 != nil:
			collectMetric(c.airPurifierPM25, float64(*reported.PM25))
		case reported.PM25Approximate != nil:
			collectMetric(c.airPurifierPM25, float64(*reported.PM25Approximate))
		}
		maybeCollectIntMetric(c.airPurifierPM10, reported.PM10)

		if reported.TVOC != nil {
			collectMetric(c.airPurifierTVOC, float64(*reported.TVOC))
			temperature := 25
			if reported.Temp != nil {
				temperature = *reported.Temp
			}
			vocDensity := tvocPPBToVocDensity(*reported.TVOC, temperature, c.options.MolecularWeight)
			collectMetric(c.airPurifierVOCDensity, round(vocDensity, 2))
		}

		var co2 *int
		switch {
		case reported.CO2 != nil && reported.ECO2 != nil:
			if reported.Metadata.ECO2.LastUpdated.After(reported.Metadata.CO2.LastUpdated) {
				co2 = reported.ECO2
			} else {
				co2 = reported.CO2
			}
		case reported.ECO2 != nil:
			co2 = reported.ECO2
		case reported.CO2 != nil:
			co2 = reported.CO2
		}
		maybeCollectIntMetric(c.airPurifierCO2, co2)
	}

	log.Println("Metrics collected.")
}

func (c *Collector) Close() error {
	c.cancel()
	return nil
}

// workmode converts the workmode string to a float64.
func workmode(s string) float64 {
	switch s {
	case "PowerOff":
		return 0
	case "Manual":
		return 1
	case "Auto":
		return 2
	case "Quiet": // Pure 500.
		return 3
	default:
		return -1
	}
}

// TODO(mafredri): Fix signal strength mapping, these are just guesses.
// Since Pure A9 reports both RSSI and signal strength string, we could
// map the signal, however, the signal strength string seems static
// until rebooted, at least on firmware 3.0.1. Probably a bug.
func signalStrengthToRSSI(s string) float64 {
	switch s {
	case "EXCELLENT": // [+, -50] dBm
		return -40
	case "GOOD": // [-50, -60] dBm
		return -50
	case "FAIR": // [-60, -70] dBm
		return -60
	case "WEAK": // [-70, -] dBm
		return -70
	default:
		return 0
	}
}

func fanspeed(model string, speed int) (perc float64, max float64, ok bool) {
	// Electrolux models are PURE/WELL, AEG models are AX.
	switch model {
	case "PUREA9", "AX9":
		max = 9
	case "WELLA5", "AX5", "WELLA7", "AX7":
		max = 5
	// This is a guess, I haven't seen these.
	case "FLOWA3", "AX3":
		max = 3
	// Electrolux Pure 500, the AEG counterpart is Pure 5000, however, it's
	// modelname is unknown. Note that the API returns "Muju" here, which is
	// the project name (weird), we'll check both just in case.
	case "Muju", "PURE500":
		max = 3
	default:
		return 0, 0, false
	}

	return float64(speed) / max, max, true
}

// tvocPPBToVocDensity converts TVOC in parts per billion (ppb) to VOC density
// (μg/m^3). This function is based on the following formula:
//
//	VOC density (μg/m^3) = P * MW * ppb / R * (K + T°C)
//
// Where:
//   - P is the standard atmospheric pressure in kPa (1 atm = 101.325 kPa)
//   - MW is the molecular weight of the gas in g/mol
//   - ppb is the TVOC in parts per billion
//   - R is the ideal gas constant
//   - K is the standard temperature in Kelvin (0°C)
//   - T is the provided temperature (in Celsius)
func tvocPPBToVocDensity(ppb, temperature int, molecularWeight float64) float64 {
	return (101.325 * molecularWeight * float64(ppb)) / (8.31446261815324 * (273.15 + float64(temperature)))
}

func round(f float64, decimals int) float64 {
	shift := math.Pow(10, float64(decimals))
	return math.Round(f*shift) / shift
}

func boolToFloat64(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

func maybe[T any](s *T) T {
	var empty T
	if s == nil {
		return empty
	}
	return *s
}
