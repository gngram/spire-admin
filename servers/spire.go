package servers

/*
import (
	"context"
	"fmt"

	"github.com/gngram/spire_admin/config"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
)

// GetAdminSpiffeID fetches the SPIFFE ID for the admin workload from the agent socket.
func GetAdminSpiffeID(ctx context.Context, configPath string) (string, error) {
	appConfig, err := config.Load(configPath)
	if err != nil {
		return "", fmt.Errorf("could not load config: %w", err)
	}

	if appConfig.ParentSocket == "" {
		return "", fmt.Errorf("parent_socket not specified in config")
	}

	source, err := workloadapi.NewX509Source(ctx, workloadapi.WithClientOptions(workloadapi.WithAddr("unix://"+appConfig.ParentSocket)))
	if err != nil {
		return "", fmt.Errorf("unable to create X.509 source: %w", err)
	}
	defer source.Close()

	svid, err := source.GetX509SVID()
	if err != nil {
		return "", fmt.Errorf("unable to get X.509 SVID: %w", err)
	}

	return svid.ID.String(), nil
}
*/
