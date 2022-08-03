package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"github.com/gorilla/handlers"
	"github.com/judwhite/go-svc"
	"github.com/korylprince/go-infinias-api"
	"github.com/korylprince/go-infinias-api/api"
	"github.com/korylprince/go-infinias-api/cmd/infinias-api/service"
	"github.com/korylprince/go-infinias-api/db"
	"gopkg.in/yaml.v3"
)

var DefaultRoot = filepath.Clean(os.Getenv("ProgramFiles") + "/infinias-api/")

var ServiceConfig = &service.ServiceConfig{
	ExecPath:    filepath.Join(DefaultRoot, "infinias-api.exe"),
	LogPath:     filepath.Join(DefaultRoot, "logs", "infinias-api.log"),
	Name:        "infinias-api",
	DisplayName: "Infinias API (Go)",
}

func run(w io.Writer) error {
	f, err := os.Open(filepath.Join(DefaultRoot, "config.yaml"))
	if err != nil {
		return fmt.Errorf("could not open config: %w", err)
	}

	config := new(Config)
	if err = yaml.NewDecoder(f).Decode(config); err != nil {
		f.Close()
		return fmt.Errorf("could not parse config: %w", err)
	}
	f.Close()

	apiConn, err := api.NewConn(config.API.Prefix, config.API.Username, config.API.Password)
	if err != nil {
		return fmt.Errorf("could not create api conn: %w", err)
	}

	query := url.Values{}
	query.Add("database", config.DB.Database)

	host := config.DB.Host
	if config.DB.Port != 0 {
		host = fmt.Sprintf("%s:%d", host, config.DB.Port)
	}

	u := &url.URL{
		Scheme:   "sqlserver",
		User:     url.UserPassword(config.DB.Username, config.DB.Password),
		Host:     host,
		Path:     config.DB.Instance,
		RawQuery: query.Encode(),
	}

	dbConn, err := db.NewConn(u.String())
	if err != nil {
		return fmt.Errorf("could not create db conn: %w", err)
	}

	s := &infinias.Service{
		APIConn: apiConn,
		DBConn:  dbConn,
		Log:     func(msg string) { log.Println(msg) },
		APIKey:  config.HTTP.APIKey,
	}

	mux := http.NewServeMux()
	mux.Handle("/", http.StripPrefix("/api/1.0", s.Handler()))
	log.Println("Listening on", config.HTTP.ListenAddr)
	return http.ListenAndServe(config.HTTP.ListenAddr, handlers.CombinedLoggingHandler(w, mux))
}

func main() {
	flInstall := flag.Bool("install", false, "install as service to "+DefaultRoot)
	flUninstall := flag.Bool("uninstall", false, "uninstall service")
	flag.Parse()

	if *flInstall {
		if err := ServiceConfig.Install(); err != nil {
			fmt.Println("could not install service:", err)
			os.Exit(1)
		}
		fmt.Println("service installed successfully")
		return
	}

	if *flUninstall {
		if err := ServiceConfig.Uninstall(); err != nil {
			fmt.Println("could not uninstall service:", err)
			os.Exit(1)
		}
		fmt.Println("service uninstalled successfully")
		return
	}

	s := ServiceConfig.Service(run)
	if err := svc.Run(s); err != nil {
		if err == service.ErrNotWindowsService {
			log.Println("not started as windows service; running in terminal")
			log.Println(run(os.Stdout))
			return
		}
		log.Println("could not start service:", err)
		os.Exit(1)
	}
}
