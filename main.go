package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"runtime"
	"strconv"

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
			log.Println("Query Error ", err)
			return
		}
		col, _ := rows.Columns()
		m.Results[0].Series[0].Columns = col
		data := PackageData(rows)
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
			return
		}
		fmt.Fprintf(w, "%s", b)
	}
}

func PackageData(rows *sql.Rows) []map[string]string {
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
