# HackCLI
#### Slack terminal client for Hack Clubbers
Faster than alt-tabbing to an already slow Slack. Check channels or DMs, ping staff or do whatever the heck you want all without leaving your IDE.
*You won't want to go back*


https://github.com/user-attachments/assets/d84b1325-e834-456d-beb3-e824d3b1f326


## Installation guide
In case HackCLI gets detected as virus do know that it's a false positive. Installing with go install should always work tho.

### macOS & Linux (via Homebrew)

If you're on macOS or Linux, the easiest way to install is using [Homebrew](https://brew.sh/).

```bash
brew tap Jan-Kur/homebrew-hackcli
brew install hackcli
```

To upgrade to a new version later, simply run:

```bash
brew upgrade hackcli
```

### Windows (via Scoop)

If you're on Windows, you can use the [Scoop](https://scoop.sh/) package manager.

```bash
scoop bucket add hackcli https://github.com/Jan-Kur/scoop-hackcli.git
scoop install hackcli
```

To upgrade to a new version later, simply run:

```bash
scoop update hackcli
```

### Go Install

If you have [Go](https://go.dev/doc/install) installed, you can just run:

```bash
go install github.com/Jan-Kur/HackCLI@latest
```

*Note: Ensure that your `$GOPATH/bin` directory is in your system's `PATH`.*

### Release binary

You can always download the latest binary directly from the releases page.

1.  Go to the [**Releases Page**](https://github.com/Jan-Kur/HackCLI/releases).
2.  Download the binary for your operating system and architecture.
3.  Extract the binary.
4.  Move the `hackcli` (or `hackcli.exe`) binary to a directory in your system's `PATH` (e.g., `/usr/local/bin` or `C:\Program Files\hackcli`).

## Setup
HackCLI mimics your slack browser session, in order to have all its functionality (learn more about that in the usage guide). Before using the app, make sure to visit the [**usage guide**](USAGE.md) to configure the app properly and learn how to use it. 

## Contributions are welcome
Let's all make this the superior slack for Hack Clubbers. If you see a bug you are more than welcome to submit an issue. If you are feeling like being a gigachad you can also fork the repo, fix a bug or add a feature yourself and submit a PR - your effort won't go unseen, trust me. Visit the [contributing guide](CONTRIBUTING.md) for more details.

