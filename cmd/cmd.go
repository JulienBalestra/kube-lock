package cmd

import (
	"flag"
	"fmt"
	"strconv"
	"time"

	"github.com/golang/glog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/JulienBalestra/kube-lock/pkg/kubelock"
	"os"
)

const programName = "kube-lock"

var viperConfig = viper.New()

// NewCommand creates a new command and return a return code
func NewCommand() (*cobra.Command, *int) {
	var verbose int
	var exitCode int

	rootCommand := &cobra.Command{
		Use:   fmt.Sprintf("%s command line", programName),
		Short: "Use this command to take a lock over a configmap",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			flag.Lookup("alsologtostderr").Value.Set("true")
			flag.Lookup("v").Value.Set(strconv.Itoa(verbose))
		},
		Args: cobra.ExactArgs(2),
		Example: fmt.Sprintf(`
%s [namespace] [configmap name]
`, programName),
		Run: func(cmd *cobra.Command, args []string) {
			l, err := newLock(args[0], args[1])
			if err != nil {
				glog.Errorf("Command returns error: %v", err)
				exitCode = 1
				return
			}
			if viperConfig.GetBool("unlock") {
				err = l.UnLock()
				if err != nil {
					glog.Errorf("Command returns error: %v", err)
					exitCode = 2
					return
				}
				return
			}
			if viperConfig.GetBool("run-once") {
				locked, err := l.LockOnce(viperConfig.GetString("reason"))
				if err != nil {
					glog.Errorf("Command returns error: %v", err)
					exitCode = 2
					return
				}
				if !locked {
					exitCode = 3
				}
				return
			}
			err = l.Lock(viperConfig.GetString("reason"))
			if err != nil {
				glog.Errorf("Command returns error: %v", err)
				exitCode = 2
				return
			}
		},
	}

	rootCommand.PersistentFlags().IntVarP(&verbose, "verbose", "v", 0, "verbose level")

	viperConfig.SetDefault("kubeconfig-path", "")
	rootCommand.PersistentFlags().String("kubeconfig-path", viperConfig.GetString("kubeconfig-path"), "kubernetes config path, leave empty for inCluster config")
	viperConfig.BindPFlag("kubeconfig-path", rootCommand.PersistentFlags().Lookup("kubeconfig-path"))

	viperConfig.SetDefault("holder-name", "")
	rootCommand.PersistentFlags().String("holder-name", viperConfig.GetString("holder-name"), "holder name, leave empty to use the hostname")
	viperConfig.BindPFlag("holder-name", rootCommand.PersistentFlags().Lookup("holder-name"))

	viperConfig.SetDefault("reason", "")
	rootCommand.PersistentFlags().String("reason", viperConfig.GetString("reason"), "holder name, leave empty to use the hostname")
	viperConfig.BindPFlag("reason", rootCommand.PersistentFlags().Lookup("reason"))

	viperConfig.SetDefault("max-holders", 1)
	rootCommand.PersistentFlags().String("max-holders", viperConfig.GetString("max-holders"), "max number of holders, must be > 0")
	viperConfig.BindPFlag("max-holders", rootCommand.PersistentFlags().Lookup("max-holders"))

	viperConfig.SetDefault("create-configmap", false)
	rootCommand.PersistentFlags().Bool("create-configmap", viperConfig.GetBool("create-configmap"), "create the configmap if not found")
	viperConfig.BindPFlag("create-configmap", rootCommand.PersistentFlags().Lookup("create-configmap"))

	viperConfig.SetDefault("polling-interval", 30*time.Second)
	rootCommand.PersistentFlags().Duration("polling-interval", viperConfig.GetDuration("polling-interval"), "interval between each lock attempt")
	viperConfig.BindPFlag("polling-interval", rootCommand.PersistentFlags().Lookup("polling-interval"))

	viperConfig.SetDefault("polling-timeout", 5*time.Minute)
	rootCommand.PersistentFlags().Duration("polling-timeout", viperConfig.GetDuration("polling-timeout"), "timeout threshold for polling")
	viperConfig.BindPFlag("polling-timeout", rootCommand.PersistentFlags().Lookup("polling-timeout"))

	viperConfig.SetDefault("run-once", false)
	rootCommand.PersistentFlags().Bool("run-once", viperConfig.GetBool("run-once"), "try to lock once, exit on error if failed")
	viperConfig.BindPFlag("run-once", rootCommand.PersistentFlags().Lookup("run-once"))

	viperConfig.SetDefault("unlock", false)
	rootCommand.PersistentFlags().Bool("unlock", viperConfig.GetBool("unlock"), "unlock the semaphore")
	viperConfig.BindPFlag("unlock", rootCommand.PersistentFlags().Lookup("unlock"))

	return rootCommand, &exitCode
}

func newLock(namespace, configmapName string) (*kubelock.KubeLock, error) {
	var err error

	holderName := viperConfig.GetString("holder-name")
	if holderName == "" {
		holderName, err = os.Hostname()
		if err != nil {
			return nil, err
		}
	}
	conf := &kubelock.Config{
		HolderName:      holderName,
		MaxHolders:      viperConfig.GetInt("max-holders"),
		Namespace:       namespace,
		ConfigmapName:   configmapName,
		PollingInterval: viperConfig.GetDuration("polling-interval"),
		PollingTimeout:  viperConfig.GetDuration("polling-timeout"),
		CreateConfigmap: viperConfig.GetBool("create-configmap"),
	}
	return kubelock.NewKubeLock(viperConfig.GetString("kubeconfig-path"), conf)
}
