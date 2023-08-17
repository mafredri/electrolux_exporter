# electrolux_exporter

Prometheus exporter for Electrolux appliances using the [Electrolux OCP API](https://github.com/mafredri/electrolux-ocp).

For now, only air purifiers are supported.

## Supported models

| Model | Brand | Notes |
| ----- | ----- | ----- |
| Pure A9 | Electrolux | Works (firmware 3.0.1) |
| Well A7 | Electrolux | Untested |
| Well A5 | Electrolux | Untested |
| Flow A3 | Electrolux | Untested |
| Pure 500 | Electrolux | Works (firmware VM213_T_02.43.25_MUJU, limited data due to lacks of sensors) |
| AX9 | AEG | Untested, same as Electrolux Pure A9 |
| AX7 | AEG | Untested, same as Electrolux Well A7 |
| AX5 | AEG | Untested, same as Electrolux Well A5 |
| Pure 5000 | AEG | Untested, same as Electrolux Pure 500 |


## Installation

```
go install github.com/mafredri/electrolux_exporter/cmd/electrolux_exporter@latest
```

## Usage

```
Usage of ./electrolux_exporter:
  -addr string
    	Listen on this address (default ":9092")
  -api-key string
    	API key (default "...")
  -brand string
    	Brand, one of: "electrolux", "aeg" (default "electrolux")
  -client-id string
    	Client ID (default "...")
  -client-secret string
    	Client secret (default "...")
  -client-state-file string
    	Path to file where client state is stored (optional) (default "electrolux_exporter_client_state.json")
  -country string
    	Country code where the exporter is running (used for API calls) (default "FI")
  -email string
    	Email address (required)
  -password string
    	Password (required)
  -voc-molecular-weight float
    	Molecular weight of gas, in g/mol. Used for TVOC (ppb) conversion VOC density (μg/m^3). Formaldehyde is 30.026 g/mol. (default 30.026)

Available environment variables:
  ELECTROLUX_EXPORTER_ADDR
  ELECTROLUX_EXPORTER_API_KEY
  ELECTROLUX_EXPORTER_BRAND
  ELECTROLUX_EXPORTER_CLIENT_ID
  ELECTROLUX_EXPORTER_CLIENT_SECRET
  ELECTROLUX_EXPORTER_CLIENT_STATE_FILE
  ELECTROLUX_EXPORTER_COUNTRY_CODE
  ELECTROLUX_EXPORTER_EMAIL
  ELECTROLUX_EXPORTER_PASSWORD
  ELECTROLUX_EXPORTER_VOC_MOLECULAR_WEIGHT
```

## Example

Run the exporter:

```
./electrolux_exporter -email user@somedomain.com -password mypassword
```

Add the following to your Prometheus config:

```yaml
scrape_configs:
  - job_name: electrolux
    scrape_interval: 30s
    static_configs:
      - targets: ['localhost:9092']
```

## Metrics

| Metric | Description |
| ------ | ----------- |
| `electrolux_appliance_connected` | Appliance is connected |
| `electrolux_appliance_workmode` | Work mode (PowerOff = 0, Manual = 1, Auto = 2, Quiet = 3) |
| `electrolux_appliance_door_open` | Door is open |
| `electrolux_appliance_ui_light` | UI light enabled |
| `electrolux_appliance_safety_lock` | Safety lock enabled |
| `electrolux_appliance_ionizer` | Ionizer enabled |
| `electrolux_appliance_filter_life` | Filter life remaining |
| `electrolux_appliance_filter_type_id` | Filter type as numeric ID |
| `electrolux_appliance_rssi` | WiFi signal strength |
| `electrolux_appliance_fanspeed` | Fan speed |
| `electrolux_appliance_fanspeed_max` | Maximum fan speed raw value |
| `electrolux_appliance_fanspeed_raw` | Fan speed (raw) |
| `electrolux_appliance_temperature` | Temperature in Celsius |
| `electrolux_appliance_humidity` | Relative humidity |
| `electrolux_appliance_pm1` | PM1 in μg/m^3 |
| `electrolux_appliance_pm25` | PM2.5 in μg/m^3 |
| `electrolux_appliance_pm10` | PM10 in μg/m^3 |
| `electrolux_appliance_co2` | CO2 |
| `electrolux_appliance_tvoc_ppb` | Total volatile organic compounds in ppb |
| `electrolux_appliance_voc_density` | Volatile organic compound density in μg/m^3) |

TODO(mafredri): Improve metrics format, perhaps add `electrolux_appliance_info`/`electrolux_appliance_status` metrics and reduce labels in other metrics.