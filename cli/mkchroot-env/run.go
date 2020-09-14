package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

var (
	flagChrootDir  = flag.String("d", "", "chroot dir")
	flagBind       = flag.String("bind", "/bin:/bin,/lib:/lib,/lib64:/lib64", "bind mount directories")
	flagExec       = flag.String("exec", "/bin/bash", "execute command")
	flagAutoUnbind = flag.Bool("auto_unbind", true, "auto unbind as cleanup")
)

var (
	cleanupUnMountTargets = []string{}
)

func bindMount(source, target string) error {
	if err := syscall.Mount(source,
		target,
		"none",
		syscall.MS_BIND|
			syscall.MS_REC,
		""); err != nil {
		return fmt.Errorf("Failed to bind mount %s -> %s: %w", source, target, err)
	}
	return nil
}

func bindMountFromFlagSingle(flag string) error {
	param := strings.SplitN(flag, ":", 3)
	if len(param) < 2 {
		return fmt.Errorf("Invalid mount definition %s", flag)
	}
	targetAbs, err := filepath.Rel("/", param[1])
	if err != nil {
		return fmt.Errorf("Mount target %s is invalid, please provide an absolute path: %v", param[1], err)
	}

	var filePerm os.FileMode
	if len(param) == 2 {
		stat, err := os.Stat(param[0])
		if err != nil {
			return fmt.Errorf("Cannot STAT %s: %w", param[0], err)
		}
		filePerm = stat.Mode()
	} else {
		perm, err := strconv.ParseUint(param[2], 8, 32)
		if err != nil {
			return fmt.Errorf("Cannot parse file mode %s: %w", param[2], err)
		}
		filePerm = os.FileMode(perm)
	}
	target := filepath.Join(*flagChrootDir, targetAbs)
	os.MkdirAll(target, filePerm)
	if err := bindMount(param[0], target); err != nil {
		return err
	}
	cleanupUnMountTargets = append(cleanupUnMountTargets, param[1])
	return nil
}

func bindMountFromFlags(flag string) error {
	mounts := strings.Split(flag, ",")
	for _, mount := range mounts {
		if err := bindMountFromFlagSingle(mount); err != nil {
			return fmt.Errorf("Cannot bind mount %s: %w", mount, err)
		}
	}
	return nil
}

func init() {
	flag.Parse()
}

func main() {
	if err := os.MkdirAll(*flagChrootDir, 0755); err != nil {
		log.Panicf("Error mkdir %s: %v", *flagChrootDir, err)
	}

	if err := bindMountFromFlags(*flagBind); err != nil {
		log.Panicf("Error creating bind mounts: %v", err)
	}

	if err := syscall.Chroot(*flagChrootDir); err != nil {
		log.Panicf("Error in chroot syscall: %v", err)
	}

	cmd := exec.Command(*flagExec, flag.Args()...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = "/"

	exitCode := 0
	if err := cmd.Run(); err != nil {
		log.Println(err)
		exitCode = cmd.ProcessState.ExitCode()
	}

	if *flagAutoUnbind {
		for _, t := range cleanupUnMountTargets {
			if err := syscall.Unmount(t, 0); err != nil {
				log.Printf("Error during cleanup: cannot unbind %s: %v", t, err)
			} else {
				if err := os.Remove(t); err != nil {
					log.Printf("Error during cleanup: cannot rm %s: %v", t, err)
				}
			}
		}
	}

	os.Exit(exitCode)
}
