package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	"github.com/spf13/viper"
)

type Results struct {
	Results []Series `json:"results"`
}
type Series struct {
	Series []Serie `json:"series"`
}
type Serie struct {
	Name    string          `json:"names"`
	Columns []string        `json:"columns"`
	Values  [][]interface{} `json:"values"`
}

func main() {
	getConfig()

	runtime.GOMAXPROCS(runtime.NumCPU())
	db, err := sql.Open(viper.GetString("DbType"), viper.GetString("DbDSN"))
	if err != nil {
		log.Fatal(err)
	}
	http.Handle("/query", handleInfluxQuery(db))
	http.HandleFunc("/", handeDefault)

	http.ListenAndServe(viper.GetString("ListenAddress"), nil)
}

func getConfig() {
	viper.SetDefault("DbType", "mysql")
	viper.SetDefault("DbDSN", "user:pass@hostname/dbname")
	viper.SetDefault("ListenAddress", "127.0.0.1:8000")
	viper.SetConfigName("config")
	viper.AddConfigPath(".")

	err := viper.ReadInConfig() // Find and read the config file
	if err != nil {             // Handle errors reading the config file
		panic(fmt.Errorf("Fatal error config file: %s \n", err))
	}
}

func handeDefault(w http.ResponseWriter, r *http.Request) {
	// The "/" pattern matches everything, so we need to check
	// that we're at the root here.
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	fmt.Fprintf(w, "OK")
}

func handleInfluxQuery(db *sql.DB) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Request-Method", "*")
		w.Header().Set("Access-Control-Allow-Methods", "OPTIONS, GET")
		w.Header().Set("Access-Control-Allow-Headers", "*")
		query := r.FormValue("q")
		if !areCredentialVerified(r) {
			log.Println("Invalid Credentials")
			http.Error(w, "401 Unauthorized", http.StatusUnauthorized)
			return
		}
		query = parseTimePart(query)

		m := Results{
			[]Series{
				Series{
					[]Serie{
						// TODO: change serie name
						Serie{"SerieNameNbr1", nil, nil},
					},
				},
			},
		}
		m.Results[0].Series[0].Values = make([][]interface{}, 0)
		rows, err := db.Query(query)
		if err != nil {
			log.Println("Query Error ", err, query)
			http.Error(w, "500 Internal Server Error", http.StatusInternalServerError)
			return
		}
		col, _ := rows.Columns()
		m.Results[0].Series[0].Columns = col
		data := FetchRow(rows)
		for row := range data {
			if data[row] != nil {
				timeVal, _ := strconv.ParseUint(data[row]["time"], 10, 64)
				var array_row []interface{}
				array_row = append(array_row, timeVal)
				for field_name, value := range data[row] {
					if field_name != "time" {
						numVal, parseErr := strconv.ParseFloat(value, 64)
						// could cast to int
						if parseErr == nil {
							array_row = append(array_row, numVal)
						} else {
							array_row = append(array_row, value)
						}

					}
				}
				m.Results[0].Series[0].Values = append(m.Results[0].Series[0].Values, array_row)
			}
		}

		b, marsherr := json.Marshal(m)
		if marsherr != nil {
			log.Println("Unable to marshalize ", marsherr)
			http.Error(w, "500 Internal Server Error", http.StatusInternalServerError)
			return
		}
		fmt.Fprintf(w, "%s", b)
	}
}

func areCredentialVerified(r *http.Request) bool {
	username := viper.GetString("influxUsername")
	password := viper.GetString("influxPassword")

	return r.FormValue("u") == username && r.FormValue("p") == password
}

func parseTimePart(query string) string {
	InfluxTimeAbbrev := map[string]string{
		"u": "microsecond",
		"s": "second",
		"m": "minute",
		"h": "hour",
		"d": "day",
		"w": "week",
	}

	match_func := "translateTimePart("
	func_start_index := strings.Index(query, match_func)
	// Match the closing parenthesis to find the contained instructions
	if func_start_index == -1 {
		return query
	}
	opened_parent := 1
	last_element_index := func_start_index + len(match_func)
	for _, c := range query[func_start_index+len(match_func):] {
		if c == '(' {
			opened_parent++
		}
		if c == ')' {
			opened_parent--
			if opened_parent == 0 {
				break
			}
		}
		last_element_index++
	}
	pre_query := query[0:func_start_index]
	post_query := query[last_element_index+1:]
	result := query[func_start_index+len(match_func) : last_element_index]

	if result != "" {

		// Match Epoch First
		re_epoch, _ := regexp.Compile(`(\d{4,})s`)
		epoch_string := re_epoch.FindAllStringSubmatch(result, -1)
		for _, v := range epoch_string {
			epoch_int, _ := strconv.ParseInt(v[1], 10, 64)
			date := time.Unix(epoch_int, 0)
			result = strings.Replace(result, v[0], "'"+date.Format(time.RFC3339Nano)+"'", 1)
		}

		for abbr, fullName := range InfluxTimeAbbrev {
			rd, _ := regexp.Compile(`(\d+)` + abbr)
			result = rd.ReplaceAllStringFunc(result, func(s string) string {
				if viper.GetString("DbType") == "mysql" {
					return "interval " + s[:len(s)-1] + " " + fullName
				} else {
					return " '" + s[:len(s)-1] + " " + fullName + "'::interval"
				}
			})
		}
	}

	return pre_query + result + post_query
}

func FetchRow(rows *sql.Rows) []map[string]string {
	columns, err := rows.Columns()
	if err != nil {
		fmt.Println("Unable to read from database", err)
		log.Println(err)
		return nil
	}
	if len(columns) == 0 {
		return nil
	}

	values := make([]sql.RawBytes, len(columns))

	scanArgs := make([]interface{}, len(values))
	for i := range values {
		scanArgs[i] = &values[i]
	}

	data := make([]map[string]string, len(values))

	// Fetch rows
	for rows.Next() {
		newRow := make(map[string]string)
		// get RawBytes from data
		err := rows.Scan(scanArgs...)
		if err != nil {
			log.Println("Unable to read from database", err)
			return nil
		}
		var value string
		for i, col := range values {
			if col == nil {
				value = "NULL"
			} else {
				value = string(col)
			}
			newRow[columns[i]] = value
		}
		data = append(data, newRow)
	}
	return data
}
