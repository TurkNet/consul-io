package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/hashicorp/vault/api"
	"github.com/spf13/cobra"
)

var (
	vaultAddr     string
	vaultPath     string
	vaultUsername string
	vaultPassword string
	vaultAuthType string
	vaultToken    string
)

var searchCmd = &cobra.Command{
	Use:   "vault-search [search-term]",
	Short: "Search for a term in Vault",
	Long: `Search for a specified term in Vault KV store. 
Example: consul-io vault-search "search-term" --vault-addr="http://vault:8200"`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		searchTerm := args[0]
		client, err := getVaultClient()
		if err != nil {
			color.Red("Vault connection error: %v", err)
			os.Exit(1)
		}

		if vaultPath != "" {
			searchInPath(client, vaultPath, searchTerm)
		} else {
			mounts, err := client.Sys().ListMounts()
			if err != nil {
				color.Red("Failed to list Vault mounts: %v", err)
				os.Exit(1)
			}

			for path, mount := range mounts {
				if mount.Type == "kv" || mount.Type == "kv-v2" {
					searchInPath(client, path, searchTerm)
				}
			}
		}
	},
}

func getVaultClient() (*api.Client, error) {
	config := api.DefaultConfig()
	if vaultAddr != "" {
		config.Address = vaultAddr
	}

	client, err := api.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create vault client: %v", err)
	}

	if vaultToken != "" {
		client.SetToken(vaultToken)
	} else if vaultAuthType == "ldap" || vaultAuthType == "userpass" {
		if vaultUsername == "" || vaultPassword == "" {
			return nil, fmt.Errorf("username and password required for authentication")
		}

		var path string
		if vaultAuthType == "ldap" {
			path = "auth/ldap/login/" + vaultUsername
		} else {
			path = "auth/userpass/login/" + vaultUsername
		}

		data := map[string]interface{}{
			"password": vaultPassword,
		}

		secret, err := client.Logical().Write(path, data)
		if err != nil {
			return nil, fmt.Errorf("authentication failed: %v", err)
		}

		client.SetToken(secret.Auth.ClientToken)
	}

	_, err = client.Auth().Token().LookupSelf()
	if err != nil {
		return nil, fmt.Errorf("invalid or missing token: %v", err)
	}

	return client, nil
}

func searchInPath(client *api.Client, path string, searchTerm string) {
	path = strings.TrimSuffix(path, "/")
	if strings.HasPrefix(path, "kv") {
		path = path + "/devops"
	}
	traversePath(client, path+"/", searchTerm)
}

func traversePath(client *api.Client, path string, searchTerm string) {
	listPath := path
	if strings.HasPrefix(listPath, "kv/") {
		listPath = strings.Replace(listPath, "kv/", "kv/metadata/", 1)
	}

	secret, err := client.Logical().List(listPath)
	if err != nil {
		return
	}

	if secret == nil {
		readPath := strings.Replace(path, "kv/metadata/", "kv/data/", 1)
		if !strings.HasPrefix(readPath, "kv/data/") && strings.HasPrefix(readPath, "kv/") {
			readPath = strings.Replace(readPath, "kv/", "kv/data/", 1)
		}
		searchInSecret(client, readPath, searchTerm)
		return
	}

	if secret.Data == nil {
		return
	}

	if keys, ok := secret.Data["keys"].([]interface{}); ok {
		for _, key := range keys {
			keyStr := key.(string)
			newPath := path + keyStr
			if strings.HasSuffix(keyStr, "/") {
				traversePath(client, newPath, searchTerm)
			} else {
				readPath := strings.Replace(newPath, "kv/metadata/", "kv/data/", 1)
				if !strings.HasPrefix(readPath, "kv/data/") && strings.HasPrefix(readPath, "kv/") {
					readPath = strings.Replace(readPath, "kv/", "kv/data/", 1)
				}
				searchInSecret(client, readPath, searchTerm)
			}
		}
	}
}

func searchInSecret(client *api.Client, path string, searchTerm string) {
	secret, err := client.Logical().Read(path)
	if err != nil {
		return
	}

	if secret == nil || secret.Data == nil {
		return
	}

	data := secret.Data
	if subData, ok := data["data"].(map[string]interface{}); ok {
		data = subData
	}

	matchedFields := make(map[string]interface{})
	for key, value := range data {
		if valueStr, ok := value.(string); ok {
			if strings.Contains(strings.ToLower(valueStr), strings.ToLower(searchTerm)) {
				matchedFields[key] = value
			}
		}
	}

	if len(matchedFields) > 0 {
		color.Green("\nFOUND - Path: %s", path)
		for key, value := range matchedFields {
			color.Yellow("%s: ", key)
			color.Red("%v", value)
			fmt.Println()
		}
		fmt.Println("----------------------------------------")
	}
}

func init() {
	searchCmd.Flags().StringVar(&vaultAddr, "vault-addr", "", "Vault server address (e.g. http://vault:8200)")
	searchCmd.Flags().StringVar(&vaultPath, "path", "", "Search in specific path (optional)")
	searchCmd.Flags().StringVar(&vaultUsername, "username", "", "Vault username")
	searchCmd.Flags().StringVar(&vaultPassword, "password", "", "Vault password")
	searchCmd.Flags().StringVar(&vaultAuthType, "auth-type", "", "Authentication type (ldap)")
	searchCmd.Flags().StringVar(&vaultToken, "token", "", "Vault token")
	rootCmd.AddCommand(searchCmd)
}
