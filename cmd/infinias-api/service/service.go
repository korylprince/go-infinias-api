package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/judwhite/go-svc"
)

var ErrNotWindowsService = errors.New("process not started as Windows service")

// ServiceConfig holds information to create a Windows service
type ServiceConfig struct {
	ExecPath    string
	LogPath     string
	Name        string
	DisplayName string
}

// Install installs the service executable, creates the Windows service, and starts it
func (s *ServiceConfig) Install() error {
	// create service directory
	if err := os.MkdirAll(filepath.Join(filepath.Dir(s.ExecPath), "logs"), 0644); err != nil {
		return fmt.Errorf("could not create service executable directory: %w", err)
	}

	// create log directory
	if err := os.MkdirAll(filepath.Dir(s.LogPath), 0644); err != nil {
		return fmt.Errorf("could not create service logs directory: %w", err)
	}

	// stop remove old service
	exec.Command(`C:\Windows\System32\sc`, "stop", s.Name).Run()
	exec.Command(`C:\Windows\System32\sc`, "delete", s.Name).Run()

	// wait for service to stop
	time.Sleep(time.Second)

	// copy self to service directory
	r, err := os.Open(os.Args[0])
	if err != nil {
		return fmt.Errorf("could not read service executable: %w", err)
	}
	defer r.Close()
	w, err := os.Create(s.ExecPath)
	if err != nil {
		return fmt.Errorf("could not create service executable: %w", err)
	}
	defer w.Close()
	if _, err = w.ReadFrom(r); err != nil {
		return fmt.Errorf("could not copy service executable: %w", err)
	}
	if err = w.Close(); err != nil {
		return fmt.Errorf("could not close service executable: %w", err)
	}

	// create and start service
	cmd := exec.Command(`C:\Windows\System32\sc`, "create", s.Name, "start=", "auto", "binPath=", s.ExecPath, "DisplayName=", s.DisplayName)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("could not create service: %w", err)
	}

	cmd = exec.Command(`C:\Windows\System32\sc`, "start", s.Name)
	if err = cmd.Run(); err != nil {
		return fmt.Errorf("could not start service: %w", err)
	}

	return nil
}

// Uninstall uninstalls the service
func (s *ServiceConfig) Uninstall() error {
	cmd := exec.Command(`C:\Windows\System32\sc`, "stop", s.Name)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("could not stop service: %w", err)
	}
	cmd = exec.Command(`C:\Windows\System32\sc`, "delete", s.Name)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("could not delete service: %w", err)
	}

	return nil
}

// Service returns a new Service for use with svc.Run
func (s *ServiceConfig) Service(main func(w io.Writer) error) *Service {
	ctx, cancel := context.WithCancel(context.Background())
	return &Service{main: main, logPath: s.LogPath, ctx: ctx, cancel: cancel}
}

// Service implements svc.Service
type Service struct {
	main    func(io.Writer) error
	logPath string
	fi      *os.File
	ctx     context.Context
	cancel  context.CancelFunc
}

// Context implements svc.Context
func (s *Service) Context() context.Context {
	return s.ctx
}

// Init implements svc.Service
func (s *Service) Init(env svc.Environment) error {
	if !env.IsWindowsService() {
		return ErrNotWindowsService
	}

	// set up log file
	fi, err := os.OpenFile(s.logPath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return fmt.Errorf("could not open log file: %w", err)
	}
	s.fi = fi
	log.SetOutput(fi)

	return nil
}

// Start implements svc.Service
func (s *Service) Start() error {
	log.Println("starting service")
	go func() {
		if err := DefaultRetryStrategy.Retry(func() error {
			return s.main(s.fi)
		}); err != nil {
			log.Println("service retries exhausted:", err)
		}
		s.cancel()
	}()
	return nil
}

// Stop implements svc.Service
func (s *Service) Stop() error {
	log.Println("stopping service")
	s.fi.Sync()
	return nil
}
