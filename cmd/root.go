// Copyright © 2019 NAME HERE <andreas.fritzler@gmail.com>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/afritzler/help-skill/pkg/types"
	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

const HelpSearchAPIURL = "https://help.sap.com/http.svc/search"
const HelpBaseURL = "https://help.sap.com"

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "help-skill",
	Short: "Todo",
	Long:  `Todo`,
	Run: func(cmd *cobra.Command, args []string) {
		log.Println("starting to serve ...")
		port := os.Getenv("PORT")
		if port == "" {
			port = "8080"
		}
		registerHandlers()
		log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", port), nil))
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.help.yaml)")
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// Search config in home directory with name ".help" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigName(".help")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}

func registerHandlers() {
	http.HandleFunc("/search", searchHandler)
	http.HandleFunc("/", returnOK)
}

func returnOK(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func searchHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	case "POST":
		body, err := ioutil.ReadAll(r.Body)
		defer r.Body.Close()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		var request types.Request
		err = json.Unmarshal(body, &request)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		responseType := request.ResponseType
		var replies []interface{}
		for _, product := range request.Products {
			client := http.Client{
				Timeout: time.Second * 10,
			}

			req, err := http.NewRequest(http.MethodGet, HelpSearchAPIURL, nil)
			if err != nil {
				log.Fatal(err)
			}

			req.Header.Set("User-Agent", "help-skill")

			q := req.URL.Query()
			q.Add("state", "PRODUCTION")
			q.Add("language", "en-US")
			q.Add("product", product.Name)
			q.Add("q", request.Converstation.Memory.Query)
			q.Add("version", product.Version)
			req.URL.RawQuery = q.Encode()

			res, getErr := client.Do(req)
			if getErr != nil {
				log.Fatal(getErr)
			}

			body, readErr := ioutil.ReadAll(res.Body)
			if readErr != nil {
				log.Fatal(readErr)
			}
			defer r.Body.Close()

			response := types.Response{}
			err = json.Unmarshal([]byte(body), &response)
			if err != nil {
				println(err)
				return
			}
			var max int
			if product.MaxResults > len(response.Data.Results) {
				max = len(response.Data.Results)
			} else {
				max = product.MaxResults
			}
			switch responseType {
			case types.BUTTONSTYPE:
				var buttons []types.Button
				for i := 0; i < max; i++ {
					r := response.Data.Results[i]
					buttons = append(buttons, types.Button{
						Title: r.Title,
						Type:  "web_url",
						Value: HelpBaseURL + r.URL,
					})
				}
				replies = append(replies, types.Buttons{
					Type: types.BUTTONSTYPE,
					Content: types.ButtonsContent{
						Title:   "Results for: " + product.Name,
						Buttons: buttons,
					},
				})
			default:
				fmt.Printf("not implemented\n")
			}
		}
		output, err := json.Marshal(types.Replies{Replies: replies})
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Header().Set("content-type", "application/json")
		w.Write(output)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		w.Write([]byte("StatusMethodNotAllowed"))
	}
}