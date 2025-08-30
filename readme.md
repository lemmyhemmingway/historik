Historik

historik is a simple command-line tool that lets you quickly and efficiently search your Zsh command history. It works by integrating with fzf to provide an interactive, fuzzy-finder interface for your commands.
Features

    Intelligent Search: Removes duplicate entries from your command history and keeps the most recent one.

    Multi-line Command Support: Correctly parses commands that span multiple lines.

    Timestamp Sorting: Sorts commands from newest to oldest.

    FZF Integration: Provides a powerful, keyboard-driven interface for fast searching.

Installation
Prerequisites

    Go language installed (version 1.18 or later).

    fzf installed on your system.

Compiling the Command

Download the source code and compile it by running the following command:

go build -o historik


Adding the Executable to Your PATH

You can run the command from anywhere by moving the created historik file to a directory in your PATH.

mv historik /usr/local/bin/


Usage

To search your command history, simply type historik in your terminal.

historik


After selecting a command from the interface, it will be automatically executed in your current shell.
Supported Shell

historik only supports the Zsh shell. For the command history to be read correctly, your HISTFILE environment variable must be set or the default .zsh_history file must exist.
