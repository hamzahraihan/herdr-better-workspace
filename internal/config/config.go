package config

import (
	"os"
	"path/filepath"
)

// Template represents a workspace starter template.
type Template struct {
	ID          string
	Name        string
	Description string
	Files       map[string]string
}

// AvailableTemplates returns the supported project scaffolding templates.
func AvailableTemplates() []Template {
	return []Template{
		{
			ID:          "blank",
			Name:        "Blank / Custom",
			Description: "Empty directory ready for any project setup",
			Files: map[string]string{
				"README.md": "# {{WORKSPACE_NAME}}\n\nCreated with `herdr-better-workspace`.\n",
			},
		},
		{
			ID:          "go",
			Name:        "Go (Golang)",
			Description: "Go module with main.go and basic .gitignore",
			Files: map[string]string{
				"go.mod": "module {{WORKSPACE_NAME}}\n\ngo 1.24\n",
				"main.go": `package main

import "fmt"

func main() {
	fmt.Println("Hello from {{WORKSPACE_NAME}}!")
}
`,
				".gitignore": "/bin/\n/dist/\n*.exe\nvendor/\n",
			},
		},
		{
			ID:          "node-ts",
			Name:        "TypeScript / Node",
			Description: "Modern TypeScript project with Node setup",
			Files: map[string]string{
				"package.json": `{
  "name": "{{WORKSPACE_NAME}}",
  "version": "1.0.0",
  "description": "Workspace created with herdr-better-workspace",
  "main": "dist/index.js",
  "scripts": {
    "build": "tsc",
    "start": "node dist/index.js",
    "dev": "ts-node src/index.ts"
  },
  "keywords": [],
  "author": "",
  "license": "MIT"
}
`,
				"tsconfig.json": `{
  "compilerOptions": {
    "target": "ES2022",
    "module": "commonjs",
    "rootDir": "./src",
    "outDir": "./dist",
    "strict": true,
    "esModuleInterop": true,
    "skipLibCheck": true,
    "forceConsistentCasingInFileNames": true
  }
}
`,
				"src/index.ts": `console.log("Hello from {{WORKSPACE_NAME}}!");
`,
				".gitignore": "node_modules/\ndist/\n.env\n",
			},
		},
		{
			ID:          "python",
			Name:        "Python",
			Description: "Python application with pyproject.toml & main.py",
			Files: map[string]string{
				"pyproject.toml": `[project]
name = "{{WORKSPACE_NAME}}"
version = "0.1.0"
description = "Workspace created with herdr-better-workspace"
readme = "README.md"
requires-python = ">=3.10"
dependencies = []
`,
				"main.py": `def main():
    print("Hello from {{WORKSPACE_NAME}}!")

if __name__ == "__main__":
    main()
`,
				"README.md":  "# {{WORKSPACE_NAME}}\n\nPython workspace created with `herdr-better-workspace`.\n",
				".gitignore": "__pycache__/\n*.py[cod]\n*$py.class\n.venv/\nenv/\n",
			},
		},
		{
			ID:          "rust",
			Name:        "Rust",
			Description: "Standard Cargo package with binary entry point",
			Files: map[string]string{
				"Cargo.toml": `[package]
name = "{{WORKSPACE_NAME}}"
version = "0.1.0"
edition = "2021"

[dependencies]
`,
				"src/main.rs": `fn main() {
    println!("Hello from {{WORKSPACE_NAME}}!");
}
`,
				".gitignore": "/target\nCargo.lock\n",
			},
		},
	}
}

// DefaultWorkspaceBaseDir returns a reasonable default parent directory for new workspaces.
func DefaultWorkspaceBaseDir() string {
	home, err := os.UserHomeDir()
	if err == nil && home != "" {
		projectsDir := filepath.Join(home, "Projects")
		// If ~/Projects exists, use it; otherwise default to home or current dir
		if info, err := os.Stat(projectsDir); err == nil && info.IsDir() {
			return projectsDir
		}
		workspacesDir := filepath.Join(home, "workspaces")
		if info, err := os.Stat(workspacesDir); err == nil && info.IsDir() {
			return workspacesDir
		}
		return home
	}
	cwd, err := os.Getwd()
	if err == nil {
		return cwd
	}
	return "."
}
