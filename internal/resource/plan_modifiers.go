package resource

import (
	"errors"
	"fmt"
	"net"

	"github.com/velodb/terraform-provider-velodb/internal/client"
)

// userError extracts a user-friendly message from an API or network error.
func userError(action string, err error) (string, string) {
	var apiErr *client.APIError
	if errors.As(err, &apiErr) {
		return fmt.Sprintf("Error %s", action), apiErr.UserMessage()
	}

	var netErr *net.OpError
	if errors.As(err, &netErr) {
		return fmt.Sprintf("Error %s", action),
			fmt.Sprintf("Cannot reach the VeloDB API: %v\n\nVerify your host configuration is correct and the API is reachable.", netErr)
	}

	return fmt.Sprintf("Error %s", action), err.Error() + "\n\nFix the configuration and re-run terraform apply."
}
