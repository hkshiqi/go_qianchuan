package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/CriarBrand/qianchuan-backend/pkg/tenantstore"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	store, err := openStore()
	if err != nil {
		fatal(err)
	}

	switch os.Args[1] {
	case "create":
		if err := createTenant(store, os.Args[2:]); err != nil {
			fatal(err)
		}
	case "add-account":
		if err := addAccount(store, os.Args[2:]); err != nil {
			fatal(err)
		}
	case "remove-account":
		if err := removeAccount(store, os.Args[2:]); err != nil {
			fatal(err)
		}
	case "delete":
		if err := deleteTenant(store, os.Args[2:]); err != nil {
			fatal(err)
		}
	case "list":
		if err := listTenants(store); err != nil {
			fatal(err)
		}
	default:
		usage()
		os.Exit(2)
	}
}

func openStore() (*tenantstore.Store, error) {
	path := strings.TrimSpace(os.Getenv("TENANT_STORE_PATH"))
	if path == "" {
		path = "data/tenants.json"
	}
	return tenantstore.New(path, strings.TrimSpace(os.Getenv("TENANT_ENCRYPTION_KEY")))
}

func createTenant(store *tenantstore.Store, args []string) error {
	flags := flag.NewFlagSet("create", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	id := flags.String("id", "", "tenant id (letters, digits, underscore, hyphen)")
	name := flags.String("name", "", "tenant display name")
	advertiserID := flags.String("advertiser-id", "", "Qianchuan advertiser id")
	awemeID := flags.String("aweme-id", "0", "Douyin aweme id")
	liveRoomID := flags.String("live-room-id", "0", "live room id")
	accountsFile := flags.String("accounts-file", "", "JSON array of additional account bindings")
	keyFile := flags.String("key-file", "", "write API key to this 0600 file")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*id) == "" || strings.TrimSpace(*name) == "" {
		return errors.New("--id and --name are required")
	}
	advertiser, err := parseID(*advertiserID, "--advertiser-id")
	if err != nil {
		return err
	}
	aweme, err := parseOptionalID(*awemeID, "--aweme-id")
	if err != nil {
		return err
	}
	room, err := parseOptionalID(*liveRoomID, "--live-room-id")
	if err != nil {
		return err
	}
	accounts, err := parseAccountsFile(*accountsFile)
	if err != nil {
		return err
	}
	accountSet := append([]tenantstore.AccountBinding{{
		AdvertiserID: advertiser,
		AwemeID:      aweme,
		LiveRoomID:   room,
	}}, accounts...)
	seenAccounts := make(map[int64]bool)
	for _, account := range accountSet {
		if seenAccounts[account.AdvertiserID] {
			return fmt.Errorf("duplicate advertiser id %d", account.AdvertiserID)
		}
		seenAccounts[account.AdvertiserID] = true
	}
	accounts = accountSet

	rawKey, err := generateAPIKey()
	if err != nil {
		return fmt.Errorf("generate API key: %w", err)
	}
	tenant := tenantstore.Tenant{
		ID:           strings.TrimSpace(*id),
		Name:         strings.TrimSpace(*name),
		AdvertiserID: advertiser,
		AwemeID:      aweme,
		LiveRoomID:   room,
		Accounts:     accounts,
	}
	if err := store.Register(tenant, rawKey); err != nil {
		return err
	}

	if strings.TrimSpace(*keyFile) != "" {
		if err := writeKeyFile(*keyFile, rawKey); err != nil {
			revokeErr := store.Delete(tenant.ID)
			if revokeErr != nil {
				return fmt.Errorf("write key file: %v; revoke tenant after failure: %w", err, revokeErr)
			}
			return err
		}
		fmt.Printf("created tenant %s; API key written to %s\n", tenant.ID, *keyFile)
		return nil
	}
	fmt.Printf("created tenant %s\n%s\n", tenant.ID, rawKey)
	return nil
}

func addAccount(store *tenantstore.Store, args []string) error {
	flags := flag.NewFlagSet("add-account", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	id := flags.String("id", "", "tenant id")
	advertiserID := flags.String("advertiser-id", "", "Qianchuan advertiser id")
	awemeID := flags.String("aweme-id", "0", "Douyin aweme id")
	liveRoomID := flags.String("live-room-id", "0", "live room id")
	if err := flags.Parse(args); err != nil {
		return err
	}
	tenantID := strings.TrimSpace(*id)
	if tenantID == "" {
		return errors.New("--id is required")
	}
	advertiser, err := parseID(*advertiserID, "--advertiser-id")
	if err != nil {
		return err
	}
	aweme, err := parseOptionalID(*awemeID, "--aweme-id")
	if err != nil {
		return err
	}
	room, err := parseOptionalID(*liveRoomID, "--live-room-id")
	if err != nil {
		return err
	}
	if err := store.UpdateBinding(tenantID, advertiser, aweme, room); err != nil {
		return err
	}
	fmt.Printf("tenant %s now includes advertiser %d\n", tenantID, advertiser)
	return nil
}

func removeAccount(store *tenantstore.Store, args []string) error {
	flags := flag.NewFlagSet("remove-account", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	id := flags.String("id", "", "tenant id")
	advertiserID := flags.String("advertiser-id", "", "Qianchuan advertiser id")
	if err := flags.Parse(args); err != nil {
		return err
	}
	tenantID := strings.TrimSpace(*id)
	if tenantID == "" {
		return errors.New("--id is required")
	}
	advertiser, err := parseID(*advertiserID, "--advertiser-id")
	if err != nil {
		return err
	}
	if err := store.RemoveAccount(tenantID, advertiser); err != nil {
		return err
	}
	fmt.Printf("tenant %s no longer includes advertiser %d\n", tenantID, advertiser)
	return nil
}

func parseAccountsFile(path string) ([]tenantstore.AccountBinding, error) {
	if strings.TrimSpace(path) == "" {
		return nil, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read accounts file: %w", err)
	}
	var accounts []tenantstore.AccountBinding
	if err := json.Unmarshal(raw, &accounts); err != nil {
		return nil, fmt.Errorf("parse accounts file: %w", err)
	}
	seen := make(map[int64]bool)
	for _, account := range accounts {
		if account.AdvertiserID <= 0 {
			return nil, errors.New("accounts file contains an invalid advertiser_id")
		}
		if seen[account.AdvertiserID] {
			return nil, fmt.Errorf("accounts file contains duplicate advertiser_id %d", account.AdvertiserID)
		}
		seen[account.AdvertiserID] = true
	}
	return accounts, nil
}

func listTenants(store *tenantstore.Store) error {
	tenants, err := store.List()
	if err != nil {
		return err
	}
	writer := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(writer, "TENANT\tNAME\tADVERTISER\tAWEME\tLIVE_ROOM\tUPDATED")
	for _, tenant := range tenants {
		for index, account := range tenant.AccountList() {
			tenantID, name := "", ""
			if index == 0 {
				tenantID, name = tenant.ID, tenant.Name
			}
			fmt.Fprintf(
				writer,
				"%s\t%s\t%d\t%d\t%d\t%s\n",
				tenantID,
				name,
				account.AdvertiserID,
				account.AwemeID,
				account.LiveRoomID,
				tenant.UpdatedAt.Format(time.RFC3339),
			)
		}
	}
	return writer.Flush()
}

func deleteTenant(store *tenantstore.Store, args []string) error {
	flags := flag.NewFlagSet("delete", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	id := flags.String("id", "", "tenant id")
	if err := flags.Parse(args); err != nil {
		return err
	}
	tenantID := strings.TrimSpace(*id)
	if tenantID == "" {
		return errors.New("--id is required")
	}
	if err := store.Delete(tenantID); err != nil {
		return err
	}
	for _, tokenDir := range []string{
		filepath.Join("/opt/go-qianchuan/data/tenants", tenantID),
		filepath.Join("data", "tenants", tenantID),
	} {
		if err := os.RemoveAll(tokenDir); err != nil {
			return fmt.Errorf("tenant deleted but token directory cleanup failed: %w", err)
		}
	}
	keyFiles := []string{
		filepath.Join("/opt/go-qianchuan/tenants", tenantID+".key"),
		filepath.Join("tenants", tenantID+".key"),
	}
	for _, keyFile := range keyFiles {
		if err := os.Remove(keyFile); err == nil {
			fmt.Printf("deleted tenant %s; removed %s\n", tenantID, keyFile)
			return nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("tenant deleted but key file cleanup failed: %w", err)
		}
	}
	fmt.Printf("deleted tenant %s; no managed key file removed\n", tenantID)
	return nil
}

func generateAPIKey() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return "qc_" + base64.RawURLEncoding.EncodeToString(value), nil
}

func writeKeyFile(path, key string) error {
	resolved := strings.TrimSpace(path)
	if resolved == "" {
		return errors.New("key file path is required")
	}
	if _, err := os.Stat(resolved); err == nil {
		return fmt.Errorf("refusing to overwrite existing key file: %s", resolved)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	file, err := os.OpenFile(resolved, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create key file: %w", err)
	}
	if _, err := file.WriteString(key + "\n"); err != nil {
		file.Close()
		return fmt.Errorf("write key file: %w", err)
	}
	if err := file.Chmod(0o600); err != nil {
		file.Close()
		return fmt.Errorf("secure key file: %w", err)
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return fmt.Errorf("sync key file: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close key file: %w", err)
	}
	return nil
}

func parseID(value, name string) (int64, error) {
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}
	return parsed, nil
}

func parseOptionalID(value, name string) (int64, error) {
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || parsed < 0 {
		return 0, fmt.Errorf("%s must be a non-negative integer", name)
	}
	return parsed, nil
}

func usage() {
	fmt.Fprintln(os.Stderr, `qianchuan-tenant create --id TENANT --name NAME --advertiser-id ID [--aweme-id ID] [--live-room-id ID] [--accounts-file PATH] [--key-file PATH]
qianchuan-tenant add-account --id TENANT --advertiser-id ID [--aweme-id ID] [--live-room-id ID]
qianchuan-tenant remove-account --id TENANT --advertiser-id ID
qianchuan-tenant delete --id TENANT
qianchuan-tenant list`)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "qianchuan-tenant:", err)
	os.Exit(1)
}
