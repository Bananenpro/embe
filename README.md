# embe

Embe is a programming language that compiles to [mBlock](https://makeblock.com) Scratch code.

**NOTE:** Due to some incompatibilities between the desktop and the web version of the mBlock IDE it is recommended to use the [web version](https://ide.mblock.cc/) to display and run output files of embe.

## Installation & Updating

### Windows

1. Hit <kbd>Win</kbd> + <kbd>R</kbd> to open the run prompt
2. Type `powershell` and hit OK
3. Paste the following command into the blue window and hit enter:

```powershell
iwr -useb https://raw.githubusercontent.com/Bananenpro/embe/main/install.ps1 | iex
```

#### Behind a proxy

```
iwr -Proxy http://example.com:8080 -useb https://raw.githubusercontent.com/Bananenpro/embe/main/install.ps1 | iex
```

### macOS/Linux

Paste one of the following commands into a terminal window:

#### curl

```bash
curl -L https://raw.githubusercontent.com/Bananenpro/embe/main/install.sh | bash
```

#### wget (in case curl is not installed)

```bash
wget -q --show-progress https://raw.githubusercontent.com/Bananenpro/embe/main/install.sh -O- | bash
```

## Editor Support

- LSP: [embe-ls](https://github.com/Bananenpro/embe-ls)
- VS Code: [vscode-embe](https://github.com/Bananenpro/vscode-embe)
- Vim: [vim-embe](https://github.com/Bananenpro/vim-embe)

## Building

### Prerequisites

- [Go](https://go.dev) 1.19+

```
git clone https://github.com/Bananenpro/embe
cd embe
go build
```

## License

Copyright (c) 2022 Julian Hofmann

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
