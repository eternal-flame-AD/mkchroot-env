package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"syscall"
)

var (
	flagCommand = flag.String("exec", "/bin/bash", "command to execute")
	flagBindMnt = flag.String("bind", "/bin:/lib:/lib64", "directories to bind mount")

	flagUidMapVal = &procIDMap{
		Current: []syscall.SysProcIDMap{
			{
				ContainerID: 0,
				HostID:      os.Getuid(),
				Size:        1,
			},
		},
	}
	flagGidMapVal = &procIDMap{
		Current: []syscall.SysProcIDMap{
			{
				ContainerID: 0,
				HostID:      os.Getgid(),
				Size:        1,
			},
		},
	}
	flagUid = flag.Uint("u", 0, "uid")
	flagGid = flag.Uint("g", 0, "gid")
)

func init() {
	flag.Var(flagUidMapVal, "U", "UID Map")
	flag.Var(flagGidMapVal, "G", "GID Map")

	flag.Parse()
}

type procIDMap struct {
	Current []syscall.SysProcIDMap
}

func (m *procIDMap) String() string {
	buf := bytes.NewBufferString("")
	for _, v := range m.Current {
		fmt.Fprintf(buf, "%d %d %d\n", v.ContainerID, v.HostID, v.Size)
	}
	return buf.String()
}

var parseProcIDMapRegex = regexp.MustCompile(`^(\d+) (\d+) (\d+)$`)

func (m *procIDMap) Set(new string) error {
	newList := strings.Split(new, ":")
	newCurrent := make([]syscall.SysProcIDMap, 0)
	for _, v := range newList {
		res := parseProcIDMapRegex.FindStringSubmatch(v)
		if len(res) != 4 {
			return fmt.Errorf("Invalid ProcIDMap: %s", v)
		}
		cid, _ := strconv.Atoi(res[1])
		hid, _ := strconv.Atoi(res[2])
		l, _ := strconv.Atoi(res[3])
		newCurrent = append(newCurrent, syscall.SysProcIDMap{cid, hid, l})
	}
	m.Current = newCurrent
	return nil
}

func main() {
	procattr := &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUSER |
			syscall.CLONE_NEWNS |
			syscall.CLONE_NEWUTS |
			syscall.CLONE_NEWPID |
			syscall.CLONE_NEWIPC |
			syscall.CLONE_NEWNET,
		Credential: &syscall.Credential{
			Uid: uint32(*flagUid),
			Gid: uint32(*flagGid),
		},
		UidMappings: flagUidMapVal.Current,
		GidMappings: flagGidMapVal.Current,
	}

	cmd := exec.Command(*flagCommand, flag.Args()...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = procattr

	if err := cmd.Run(); err != nil {
		log.Println(err)
		os.Exit(cmd.ProcessState.ExitCode())
	}
}
