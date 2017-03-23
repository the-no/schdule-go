package main

import (
	"github.com/BurntSushi/toml"
	"log"
)

type Config struct {
	Maxprocs        int                `toml:"maxprocs"`
	Dbinfo          map[string]*dbinfo `toml:"dbinfo"`
	ManagerPort     string             `toml:"managerport"`
	Port            string             `toml:"port"`
	Loglevel        uint8              `toml:"loglevel"`
	SchedulePidFile string             `toml:"schedule_pid_file"`
	WorkerPidFile   string             `toml:"worker_pid_file"`
	CpuProfName     string             `toml:"cpuprof"`
	MemProfName     string             `toml:"memprof"`
}

type dbinfo struct {
	Dbtype string
	Conn   string
}

func LoadConfig(configPath string) (config *Config) {

	if _, err := toml.DecodeFile(configPath, &config); err != nil {
		log.Fatal("Error reading config: ", err)
	}

	return config
}
