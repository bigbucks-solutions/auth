package cmd

import (
	"bigbucks/solution/auth/models"
	sessionstore "bigbucks/solution/auth/session_store"
	"bigbucks/solution/auth/settings"
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	mergeDryRun  bool
	mergeAddress string
)

// errDryRun rolls a dry run's transaction back.
var errDryRun = errors.New("dry run")

// userCmd groups admin actions on user accounts.
var userCmd = &cobra.Command{
	Use:   "user",
	Short: "Admin actions for user accounts",
	RunE: func(cmd *cobra.Command, args []string) error {
		return errors.New("invalid command")
	},
}

// mergeProviderAccountsCmd folds the old "<address>+google" and
// "<address>+microsoft" accounts into the one account at each address.
//
// Provider sign-ins used to make an account of their own beside a password
// account on the same address. They now sign in to the address's account, and
// merge an old one into it on the way in; this does the same for everyone at
// once, so nobody waits on their next sign-in to see their organizations
// together. Safe to run repeatedly: a merged address has nothing left to merge.
var mergeProviderAccountsCmd = &cobra.Command{
	Use:   "merge-provider-accounts",
	Short: "Merge each +google / +microsoft account into the account at its address",
	Long: `Merge every old provider account into the one account at its address.

Organization roles (the wider one where both accounts are in an organization),
passkeys, provider links, sign-in history and missing profile fields move to
the kept account; the old one is soft-deleted and its sessions end. With no
account at the address, the old one is renamed to it. Accounts that cannot be
merged safely are reported and left alone.

	auth user merge-provider-accounts --dry-run
	auth user merge-provider-accounts
	auth user merge-provider-accounts --address ana@example.com`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return mergeProviderAccounts(cmd.OutOrStdout(), models.Dbcon.Session(&gorm.Session{
			Logger: logger.Default.LogMode(logger.Silent),
		}), mergeAddress, mergeDryRun, func() sessionEnder {
			return sessionstore.NewSessionStore(settings.Current)
		})
	},
}

type sessionEnder interface {
	RevokeAllUserSessions(userID, exceptSessionID string) error
}

func mergeProviderAccounts(out io.Writer, db *gorm.DB, address string, dryRun bool,
	sessions func() sessionEnder) error {
	addresses := []string{models.NormalizeAddress(address)}
	if address == "" {
		var err error
		if addresses, err = models.LegacyProviderAddresses(db); err != nil {
			return err
		}
	}
	if len(addresses) == 0 {
		_, _ = fmt.Fprintln(out, "No +google or +microsoft accounts; nothing to merge.")
		return nil
	}

	verb, label := "merged", "merge"
	if dryRun {
		verb, label = "would merge", "dry-run"
	}
	var merged, conflicts, failures int
	var endSessions []string
	for _, address := range addresses {
		var outcome models.MergeOutcome
		err := db.Transaction(func(tx *gorm.DB) error {
			var err error
			if outcome, err = models.MergeLegacyAccounts(tx, address); err != nil {
				return err
			}
			if dryRun {
				return errDryRun
			}
			return nil
		})
		if err != nil && !errors.Is(err, errDryRun) {
			failures++
			_, _ = fmt.Fprintf(out, "FAILED   %s: %v\n", address, err)
			continue
		}
		if outcome.Renamed != "" {
			_, _ = fmt.Fprintf(out, "rename   %s -> %s (%s)\n", outcome.Renamed, address, outcome.KeptID)
		}
		for _, username := range outcome.Merged {
			_, _ = fmt.Fprintf(out, "%-8s %s -> %s (%s)\n", label, username, address, outcome.KeptID)
		}
		for _, note := range outcome.Notes {
			_, _ = fmt.Fprintf(out, "  note   %s\n", note)
		}
		for _, conflict := range outcome.Conflicts {
			_, _ = fmt.Fprintf(out, "CONFLICT %s\n", conflict)
		}
		merged += len(outcome.Merged)
		if outcome.Renamed != "" {
			merged++
		}
		conflicts += len(outcome.Conflicts)
		endSessions = append(endSessions, outcome.EndSessions...)
	}

	if !dryRun && len(endSessions) > 0 {
		store := sessions()
		for _, userID := range endSessions {
			if err := store.RevokeAllUserSessions(userID, ""); err != nil {
				failures++
				_, _ = fmt.Fprintf(out, "FAILED   ending sessions of %s: %v\n", userID, err)
			}
		}
	}

	_, _ = fmt.Fprintf(out, "\n%d addresses: %s %d accounts, %d conflicts, %d failures.\n",
		len(addresses), verb, merged, conflicts, failures)
	if failures > 0 {
		return fmt.Errorf("%d addresses failed", failures)
	}
	return nil
}

func init() {
	rootCmd.AddCommand(userCmd)
	userCmd.AddCommand(mergeProviderAccountsCmd)
	mergeProviderAccountsCmd.Flags().BoolVar(&mergeDryRun, "dry-run", false, "report what would be merged and change nothing")
	mergeProviderAccountsCmd.Flags().StringVar(&mergeAddress, "address", "", "merge only this address's accounts")
}
