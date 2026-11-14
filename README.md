# drag

**drag** instantly generates clean, token-efficient Markdown directory trees, giving AI coding assistants the perfect project context.

## 🚀 About

When working with AI coding assistants (like Cursor, GitHub Copilot, Claude, or ChatGPT), providing your project's directory structure is crucial for accurate and context-aware responses. However, traditional tools like `tree` or `find` often produce verbose, messy output that wastes valuable context tokens.

**drag** solves this by generating a clean, highly readable ASCII directory tree in Markdown format, complete with smart filtering options to exclude noisy directories (like `node_modules`, `.git`, or `build` folders).

## ✨ Features

* 🎯 **AI-Optimized**: Generates clean, token-efficient Markdown output perfect for LLM prompts.

* 🌳 **Nested or Flat**: Choose between a classic ASCII nested tree or a simple flat list.

* 🚫 **Smart Exclusions**: Completely prune directories or keep the directory name but skip walking its contents.

* 🔒 **Safe by Default**: Automatically excludes the output file itself to prevent self-inclusion loops.

* ⚙️ **Highly Configurable**: Supports hidden files, absolute paths, and deterministic output (no timestamps).

* ⚡ **Blazing Fast**: Written in Go for near-instant execution, even on large codebases.

## 📦 Installation

### Option 1: Install via Go (Recommended)

ectly to your `$GOPATH/bin`:If you have Go installed, you can install `drag` dir

```
go install github.com/pat3icki/drag@latest

```

### Option 2: Build from Source

```
git clone https://github.com/pat3icki/drag.git
cd drag
go build -o drag .
# Optionally, move the binary to your PATH (e.g., sudo mv drag /usr/local/bin/)

```

## 💻 Usage

```
drag [flags] <root-dir>

```

### Flags

| Flag | Description | Default | 
| ----- | ----- | ----- | 
| `-o, --output` | Output Markdown file path | `./DIRECTORY.md` | 
| `-e, --exclude` | Subdirectory to prune completely *(repeatable)* | None | 
| `-c, --exclude-content` | Directory to keep, but exclude its contents from walking *(repeatable)* | None | 
| `--include-hidden` | Include dotfiles and dot-directories (e.g., `.git`, `.env`) | `false` | 
| `--flat` | Emit a flat list instead of a nested ASCII tree | `false` | 
| `--absolute` | Emit absolute paths instead of root-relative paths | `false` | 
| `-q, --quiet` | Suppress per-entry warnings (e.g., missing exclude targets) | `false` | 
| `--no-timestamp` | Omit the generation timestamp from the header for deterministic output | `false` | 

### 🚦 Exit Codes

`drag` uses standard exit codes to indicate whether execution succeeded or why it terminated early, making it reliable for CI/CD pipelines and shell scripts:

| Exit Code | Meaning | Reason for Exit | 
| ----- | ----- | ----- | 
| `0` | **Success** | The directory tree was walked and rendered to the output file without fatal errors. | 
| `1` | **Runtime / I/O Error** | The root directory could not be read due to permission issues, or the output Markdown file could not be created/written. | 
| `2` | **Usage / Configuration Error** | Invalid CLI flags were passed, the `<root-dir>` argument was missing or extra arguments were supplied, or the target path does not exist or is not a directory. | 

## 📝 Examples

### 1. Basic Usage

Generate a directory tree for the current directory and save it to `DIRECTORY.md`:

```
drag .

```

### 2. Exclude Noisy Directories

Generate a tree while completely ignoring `node_modules` and `.git`:

```
drag . --exclude node_modules --exclude .git

```

### 3. Keep Directory, Skip Contents

If you want the AI to know a `dist` or `build` folder exists, but don't want to waste tokens on its hundreds of compiled files:

```
drag . --exclude-content dist --exclude-content build

```

### 4. Flat List with Absolute Paths

Generate a simple, flat list of files using absolute paths (useful for specific scripting or indexing needs):

```
drag . --flat --absolute --output files_list.md

```

### 5. Deterministic Output for Git

Generate a tree without a timestamp so the output doesn't change on every run. This is great for committing the `DIRECTORY.md` file to your repository:

```
drag . --no-timestamp --exclude .git

```

## 📄 Example Output

Running `drag .` produces a clean Markdown file like this:

````
# Directory Structure: `./my-project`

> **Generated:** 2026-11-14 10:30:00 | **Directories:** 4 | **Files:** 6

```txt
my-project/
|---- src/
|    |---- main.go
|    |---- utils/
|         |---- helper.go
|---- go.mod
|---- go.sum
|---- README.md
```

````

## 🤝 Contributing

Contributions, issues, and feature requests are welcome! Feel free to check the [issues page](https://github.com/pat3icki/drag/issues).

## 📜 License

This project is licensed under the [MIT License](LICENSE).

Made with ❤️ by [pat3icki](https://github.com/pat3icki)