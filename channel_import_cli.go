/*
Copyright (C) 2023-2026 QuantumNous
Copyright (C) 2026 CubeRouter

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/channelconfig"
)

// runChannelImportCLI implements `cuberouter channels import -f <file>`.
//
// It boots the same resources as the server (database, options, provider
// defaults), applies the declarative file, prints the plan and exits. Running
// it against a live instance is safe: the server picks up channel changes
// through its regular database sync loops.
func runChannelImportCLI(args []string) int {
	flags := flag.NewFlagSet("channels import", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	var file string
	flags.StringVar(&file, "f", "", "path to the channel bootstrap YAML file")
	flags.StringVar(&file, "file", "", "path to the channel bootstrap YAML file")
	dryRun := flags.Bool("dry-run", false, "resolve channels and models without writing to the database")
	var verbose bool
	flags.BoolVar(&verbose, "v", false, "show fetched/resolved models")
	flags.BoolVar(&verbose, "verbose", false, "show fetched/resolved models")
	flags.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: cuberouter channels import -f <config.yaml> [--dry-run] [--verbose]")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "error: unexpected arguments: %q\n", flags.Args())
		flags.Usage()
		return 2
	}
	if strings.TrimSpace(file) == "" {
		fmt.Fprintln(os.Stderr, "error: -f <config.yaml> is required")
		flags.Usage()
		return 2
	}

	if err := InitResources(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize resources: %v\n", err)
		return 1
	}
	defer func() {
		if err := model.CloseDB(); err != nil {
			fmt.Fprintf(os.Stderr, "failed to close database: %v\n", err)
		}
	}()

	result, err := controller.ApplyChannelConfigFile(file, controller.ChannelConfigApplyOptions{DryRun: *dryRun})
	if result != nil {
		printChannelConfigResult(result, verbose)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "\nchannel import failed: %v\n", err)
		return 1
	}
	return 0
}

func printChannelConfigResult(result *controller.ChannelConfigApplyResult, verbose bool) {
	if result.DryRun {
		fmt.Println("dry run: nothing was written to the database")
	}
	for _, channel := range result.Channels {
		status := channel.Action
		if channel.Error != "" {
			status = "failed"
		}
		fmt.Printf("[%s] %s  type=%d (%s)  models=%d (fetched %d)  id=%d\n",
			status, channel.Name, channel.Type, channel.TypeName, len(channel.Models), len(channel.Fetched), channel.ID)
		if channel.Error != "" {
			fmt.Printf("        error: %s\n", channel.Error)
		}
		if verbose {
			printModelGroup("models", channel.Models)
			printModelGroup("filtered out by include/exclude", missingModels(channel.Fetched, channel.Models))
			printModelGroup("manual additions (models.list)", missingModels(channel.Models, channel.Fetched))
		}
	}

	if result.Pricing.Models > 0 {
		parts := make([]string, 0, len(result.Pricing.Written))
		for _, option := range []string{"ModelRatio", "CompletionRatio", "CacheRatio", "CreateCacheRatio", "ModelPrice"} {
			if count, ok := result.Pricing.Written[option]; ok {
				parts = append(parts, fmt.Sprintf("%s=%d", option, count))
			}
		}
		if len(parts) > 0 {
			fmt.Printf("pricing: %d model(s)  %s\n", result.Pricing.Models, strings.Join(parts, "  "))
		} else {
			fmt.Printf("pricing: %d model(s) planned\n", result.Pricing.Models)
		}
	}
	if verbose && len(result.Pricing.Detail) > 0 {
		fmt.Println("prices (USD per 1M tokens, or per_request per call):")
		names := make([]string, 0, len(result.Pricing.Detail))
		for name := range result.Pricing.Detail {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			fmt.Printf("        %-40s %s\n", name, formatConfigPrice(result.Pricing.Detail[name]))
		}
	}

	warnings := append([]string{}, result.Warnings...)
	warnings = append(warnings, result.Pricing.Warnings...)
	if len(warnings) > 0 {
		sort.Strings(warnings)
		fmt.Println("warnings:")
		last := ""
		for _, warning := range warnings {
			if warning == last {
				continue
			}
			last = warning
			fmt.Printf("  - %s\n", warning)
		}
	}
}

// printModelGroup prints one indented model list, skipping empty groups.
func printModelGroup(label string, models []string) {
	if len(models) == 0 {
		return
	}
	fmt.Printf("        %s (%d):\n", label, len(models))
	for _, name := range models {
		fmt.Printf("          - %s\n", name)
	}
}

// missingModels returns the entries of want that do not appear in have.
func missingModels(want, have []string) []string {
	if len(want) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(have))
	for _, name := range have {
		seen[name] = struct{}{}
	}
	missing := make([]string, 0, len(want))
	for _, name := range want {
		if _, ok := seen[name]; !ok {
			missing = append(missing, name)
		}
	}
	return missing
}

func formatConfigPrice(price channelconfig.Price) string {
	if price.PerRequest != nil {
		return fmt.Sprintf("per_request=$%g", *price.PerRequest)
	}
	parts := make([]string, 0, 4)
	if price.Input != nil {
		parts = append(parts, fmt.Sprintf("input=$%g", *price.Input))
	}
	if price.Output != nil {
		parts = append(parts, fmt.Sprintf("output=$%g", *price.Output))
	}
	if price.CacheRead != nil {
		parts = append(parts, fmt.Sprintf("cache_read=$%g", *price.CacheRead))
	}
	if price.CacheWrite != nil {
		parts = append(parts, fmt.Sprintf("cache_write=$%g", *price.CacheWrite))
	}
	return strings.Join(parts, "  ")
}
