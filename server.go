package main

import (
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"github.com/Sirupsen/logrus"
	_ "github.com/go-sql-driver/mysql"
	"gitlab.51idc.com/hds/scheduling/manager"
	"gitlab.51idc.com/hds/scheduling/schedule"
	"gitlab.51idc.com/hds/scheduling/worker"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"runtime"
	"runtime/pprof"
	"strconv"
	"strings"
	"syscall"
)

const (
	VERSION = "0.0.1"
)

func setConfig(config *Config) (*schedule.GlobalConfigStruct, string, string) {
	maxprocs := config.Maxprocs
	port := config.Port
	managerport := config.ManagerPort
	loglevel := config.Loglevel
	cpuProfName := config.CpuProfName
	memProfName := config.MemProfName

	runtime.GOMAXPROCS(maxprocs)

	dg := schedule.DefaultGlobal()
	dg.L.Level = logrus.Level(loglevel)
	dg.Port = ":" + port
	dg.ManagerPort = ":" + managerport

	return dg, cpuProfName, memProfName
}

func main() {
	isSchedule := flag.Bool("s", false, "run a schedule instead of a worker")
	version := flag.Bool("version", false, "Output version and exit")
	flag.Parse()

	config := &Config{}
	var cpuProfName string
	var memProfName string

	if *version {
		fmt.Println(VERSION)
		os.Exit(0)
	}

	config = LoadConfig("config.toml")
	global, cpuProfName, memProfName := setConfig(config)

	if *isSchedule { // {{{
		if config.SchedulePidFile != "" {
			if err := checkAndSetPid(config.SchedulePidFile); err != nil {
				log.Fatalf(err.Error())
			}

			defer func() {
				if err := os.Remove(config.SchedulePidFile); err != nil {
					log.Fatalf("Unable to remove pidfile '%s': %s", config.SchedulePidFile, err)
				}
			}()
		}

		if cpuProfName != "" {
			profFile, err := os.Create(cpuProfName)
			if err != nil {
				log.Fatalf("Unable to write cpuprofile %s", err)
			}

			pprof.StartCPUProfile(profFile)
			defer func() {
				pprof.StopCPUProfile()
				profFile.Close()
			}()
		}

		if memProfName != "" {
			defer func() {
				profFile, err := os.Create(memProfName)
				if err != nil {
					log.Fatalf("Unable to write memprofile %s", err)
				}
				pprof.WriteHeapProfile(profFile)
				profFile.Close()
			}()
		}

		cnn, err := sql.Open(config.Dbinfo["hivedb"].Dbtype, config.Dbinfo["hivedb"].Conn)
		if err != nil {
			log.Fatalf("Unable to connect metadata database. %s", err)
		}
		global.HiveConn = cnn
		defer global.HiveConn.Close()

		cnn, err = sql.Open(config.Dbinfo["logdb"].Dbtype, config.Dbinfo["logdb"].Conn)
		if err != nil {
			log.Fatalf("Unable to connect metadata database. %s", err)
		}
		global.LogConn = cnn
		defer global.LogConn.Close()

		//初始化
		global.Schedules.InitScheduleList()
		//启动调度
		go global.Schedules.StartListener()

		//启动管理模块
		go manager.StartManager(global.Schedules)

		waitExit("Schedule")
	} else { // }}}

		if config.SchedulePidFile != "" { // {{{
			if err := checkAndSetPid(config.WorkerPidFile); err != nil {
				log.Fatalf(err.Error())
			}

			defer func() {
				if err := os.Remove(config.WorkerPidFile); err != nil {
					log.Fatalf("Unable to remove pidfile '%s': %s", config.WorkerPidFile, err)
				}
			}()
		} // }}}

		worker.ListenAndServer(global.Port)

		waitExit("Worker")
	}

}

func checkAndSetPid(pidFile string) error { // {{{
	contents, err := ioutil.ReadFile(pidFile)
	if err == nil {
		pid, err := strconv.Atoi(strings.TrimSpace(string(contents)))
		if err != nil {
			return errors.New(fmt.Sprintf("Error reading proccess id from pidfile '%s': %s", pidFile, err))
		}

		process, err := os.FindProcess(pid)

		// on Windows, err != nil if the process cannot be found
		if runtime.GOOS == "windows" {
			if err == nil {
				return errors.New(fmt.Sprintf("Process %d is already running.", pid))
			}
		} else if process != nil {
			// err is always nil on POSIX, so we have to send the process a signal to check whether it exists
			if err = process.Signal(syscall.Signal(0)); err == nil {
				return errors.New(fmt.Sprintf("Process %d is already running.", pid))
			}
		}
	}
	if err := ioutil.WriteFile(pidFile, []byte(strconv.Itoa(os.Getpid())), 0644); err != nil {
		return errors.New(fmt.Sprintf("Unable to write pidfile '%s': %s", pidFile, err))
	}
	log.Printf("Wrote pid to pidfile '%s'", pidFile)

	return nil
} // }}}

func waitExit(name string) { // {{{
	sig := make(chan os.Signal)
	// wait for sigint
	signal.Notify(sig, syscall.SIGKILL, syscall.SIGINT, syscall.SIGHUP, syscall.SIGALRM, syscall.SIGTERM)

	for {
		switch <-sig {
		case syscall.SIGKILL, syscall.SIGINT, syscall.SIGHUP, syscall.SIGALRM, syscall.SIGTERM:
			log.Printf("%s is exit.", name)
			return
		}
	}
} // }}}
