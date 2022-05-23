package main

import (
	"github.com/Azure/azure-docker-extension/pkg/vmextension"
	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"
	"os"
	"time"
	"strings"
)

type cmdFunc func(ctx *log.Context, hEnv vmextension.HandlerEnvironment, seqNum int) (msg string, err error)
type preFunc func(ctx *log.Context, seqNum int) error

type cmd struct {
	f                  cmdFunc // associated function
	name               string  // human readable string
	shouldReportStatus bool    // determines if running this should log to a .status file
	pre                preFunc // executed before any status is reported
	failExitCode       int     // exitCode to use when commands fail
}

const (
	fullName = "Microsoft.ManagedServices.ApplicationHealthLinux"
)

var (
	cmdInstall   = cmd{install, "Install", false, nil, 52}
	cmdEnable    = cmd{enable, "Enable", true, nil, 3}
	cmdUninstall = cmd{uninstall, "Uninstall", false, nil, 3}

	cmds = map[string]cmd{
		"install":   cmdInstall,
		"uninstall": cmdUninstall,
		"enable":    cmdEnable,
		"update":    {noop, "Update", true, nil, 3},
		"disable":   {noop, "Disable", true, nil, 3},
	}
)

func noop(ctx *log.Context, h vmextension.HandlerEnvironment, seqNum int) (string, error) {
	ctx.Log("event", "noop")
	return "", nil
}

func install(ctx *log.Context, h vmextension.HandlerEnvironment, seqNum int) (string, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return "", errors.Wrap(err, "failed to create data dir")
	}

	ctx.Log("event", "created data dir", "path", dataDir)
	ctx.Log("event", "installed")
	return "", nil
}

func uninstall(ctx *log.Context, h vmextension.HandlerEnvironment, seqNum int) (string, error) {
	{ // a new context scope with path
		ctx = ctx.With("path", dataDir)
		ctx.Log("event", "removing data dir", "path", dataDir)
		if err := os.RemoveAll(dataDir); err != nil {
			return "", errors.Wrap(err, "failed to delete data dir")
		}
		ctx.Log("event", "removed data dir")
	}
	ctx.Log("event", "uninstalled")
	return "", nil
}

const (
	statusMessage = "Successfully polling for application health"
)

var (
	errTerminated = errors.New("Application health process terminated")
)

func enable(ctx *log.Context, h vmextension.HandlerEnvironment, seqNum int) (string, error) {
	// parse the extension handler settings (not available prior to 'enable')
	cfg, err := parseAndValidateSettings(ctx, h.HandlerEnvironment.ConfigFolder)
	if err != nil {
		return "", errors.Wrap(err, "failed to get configuration")
	}

	var prevState HealthStatus
	probe := NewHealthProbe(ctx, &cfg)
	var (
		intervalInSeconds  = cfg.intervalInSeconds()
		numberOfProbes     = cfg.numberOfProbes()
		numberOfProbesLeft = numberOfProbes
		committedState     = Empty
	)
	intervalBetweenProbesInSeconds := time.Duration(intervalInSeconds) * time.Second

	// The committed health status (the state written to the status file) initially does not have a state
	// In order to change the state in the status file, the following must be observed:
	//  1. Healthy status observed once when committed state is unknown
	//  2. A different status is observed numberOfProbes consecutive times
	// Example: Committed state = healthy, numberOfProbes = 3
	// In order to change committed state to unhealthy, the probe needs to be unhealthy 3 consecutive times
	for {
		startTime := time.Now()
		state, err := probe.evaluate(ctx)
		if err != nil {
			ctx.Log("error", err)
		}

		if shutdown {
			return "", errTerminated
		}

		if state != committedState {
			numberOfProbesLeft--
		} else {
			numberOfProbesLeft = numberOfProbes
		}

		if prevState != state {
			ctx.Log("event", "State changed to "+strings.ToLower(string(state)))
			prevState = state
		}

		if (numberOfProbesLeft == 0) || (committedState == Empty) {
			committedState = state
			ctx.Log("event", "Committed health state is "+strings.ToLower(string(committedState)))
			numberOfProbesLeft = numberOfProbes
		}

		err = reportStatusWithSubstatus(ctx, h, seqNum, StatusSuccess, "enable", statusMessage, committedState.GetStatusType(), SubstatusKeyNameAppHealthStatus, committedState.GetSubstatusMessage(), committedState)
		if err != nil {
			ctx.Log("error", err)
		}

		endTime := time.Now()
		durationToWait := intervalBetweenProbesInSeconds - endTime.Sub(startTime)
		if durationToWait > 0 {
			time.Sleep(durationToWait)
		}

		if shutdown {
			return "", errTerminated
		}
	}
}
