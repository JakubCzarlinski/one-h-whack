# one-h-whack

A tool for renaming your files to French. I do not know why you would want to do
this.

I do not know French.

## Setup

Download go 1.25.1. Have fun: [https://go.dev/doc/install](https://go.dev/doc/install)

Please use a modern terminal that support colors and unicode characters.

Run:

`go mod tidy`

`go build ./src/main.go && ./main`

I recommend only running this on the `./test` files.

Features:
- Renames files to French.
  - Translations are done asynchronously and cached for blazing fast performance so that you rename that one file you really want to rename in no time.
- Arrow keys to navigate.
  - Left to go to parent directory.
  - Right to go into a directory.
  - Up and down to navigate the current directory.
- Press enter to rename a file or directory.
- Press q or esc to exit.
- Filter files by typing `/` followed by your search term.
  - Exit filtering by pressing `esc`.

## Example Figures

![Example1](./imgs/dir_view.png)
![Example2](./imgs/renaming.png)
