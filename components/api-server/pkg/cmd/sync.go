package cmd

import (
	"embed"
	"fmt"
	"io/fs"
	"os"

	"github.com/spf13/cobra"
)

//go:embed boilerplate/*
var boilerplateFS embed.FS

func NewSyncCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "sync",
		Short: "Synchronize TRex boilerplate (specs, Makefiles, etc) into the current project",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("Syncing TRex boilerplate to current directory...")
			
			err := fs.WalkDir(boilerplateFS, "boilerplate", func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				
				// Strip the "boilerplate/" prefix for the output path
				outPath := path[len("boilerplate"):]
				if outPath == "" || outPath == "/" {
					return nil
				}
				outPath = "." + outPath
				
				if d.IsDir() {
					return os.MkdirAll(outPath, 0755)
				}
				
				content, err := boilerplateFS.ReadFile(path)
				if err != nil {
					return err
				}
				
				fmt.Printf("  -> Writing %s\n", outPath)
				return os.WriteFile(outPath, content, 0644)
			})
			
			if err == nil {
				fmt.Println("Sync complete! Your boilerplate is now up to date with upstream TRex.")
			}
			return err
		},
	}
}
