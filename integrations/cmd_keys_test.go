// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"bytes"
	"flag"
	"io"
	"net/url"
	"os"
	"testing"

	"code.gitea.io/gitea/cmd"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"github.com/urfave/cli/v2"
)

func Test_CmdKeys(t *testing.T) {
	onGiteaRun(t, func(*testing.T, *url.URL) {
		tests := []struct {
			name           string
			args           []string
			wantErr        bool
			expectedOutput string
		}{
			{"test_empty_1", []string{"keys", "--username=git", "--type=test", "--content=test"}, true, ""},
			{"test_empty_2", []string{"keys", "-e", "git", "-u", "git", "-t", "test", "-k", "test"}, true, ""},
			{"with_key",
				[]string{"keys", "-e", "git", "-u", "git", "-t", "ssh-rsa", "-k", "AAAAB3NzaC1yc2EAAAADAQABAAABgQDWVj0fQ5N8wNc0LVNA41wDLYJ89ZIbejrPfg/avyj3u/ZohAKsQclxG4Ju0VirduBFF9EOiuxoiFBRr3xRpqzpsZtnMPkWVWb+akZwBFAx8p+jKdy4QXR/SZqbVobrGwip2UjSrri1CtBxpJikojRIZfCnDaMOyd9Jp6KkujvniFzUWdLmCPxUE9zhTaPu0JsEP7MW0m6yx7ZUhHyfss+NtqmFTaDO+QlMR7L2QkDliN2Jl3Xa3PhuWnKJfWhdAq1Cw4oraKUOmIgXLkuiuxVQ6mD3AiFupkmfqdHq6h+uHHmyQqv3gU+/sD8GbGAhf6ftqhTsXjnv1Aj4R8NoDf9BS6KRkzkeun5UisSzgtfQzjOMEiJtmrep2ZQrMGahrXa+q4VKr0aKJfm+KlLfwm/JztfsBcqQWNcTURiCFqz+fgZw0Ey/de0eyMzldYTdXXNRYCKjs9bvBK+6SSXRM7AhftfQ0ZuoW5+gtinPrnmoOaSCEJbAiEiTO/BzOHgowiM="},
				false,
				"# gitea public key\ncommand=\"" + setting.AppPath + " --config=" + util.ShellEscape(setting.CustomConf) + " serv key-1\",no-port-forwarding,no-X11-forwarding,no-agent-forwarding,no-pty ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQDWVj0fQ5N8wNc0LVNA41wDLYJ89ZIbejrPfg/avyj3u/ZohAKsQclxG4Ju0VirduBFF9EOiuxoiFBRr3xRpqzpsZtnMPkWVWb+akZwBFAx8p+jKdy4QXR/SZqbVobrGwip2UjSrri1CtBxpJikojRIZfCnDaMOyd9Jp6KkujvniFzUWdLmCPxUE9zhTaPu0JsEP7MW0m6yx7ZUhHyfss+NtqmFTaDO+QlMR7L2QkDliN2Jl3Xa3PhuWnKJfWhdAq1Cw4oraKUOmIgXLkuiuxVQ6mD3AiFupkmfqdHq6h+uHHmyQqv3gU+/sD8GbGAhf6ftqhTsXjnv1Aj4R8NoDf9BS6KRkzkeun5UisSzgtfQzjOMEiJtmrep2ZQrMGahrXa+q4VKr0aKJfm+KlLfwm/JztfsBcqQWNcTURiCFqz+fgZw0Ey/de0eyMzldYTdXXNRYCKjs9bvBK+6SSXRM7AhftfQ0ZuoW5+gtinPrnmoOaSCEJbAiEiTO/BzOHgowiM= user2@localhost\n",
			},
			{"invalid", []string{"keys", "--not-a-flag=git"}, true, "Incorrect Usage: flag provided but not defined: -not-a-flag\n\n"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				realStdout := os.Stdout //Backup Stdout
				r, w, _ := os.Pipe()
				os.Stdout = w

				set := flag.NewFlagSet("keys", 0)
				_ = set.Parse(tt.args)
				context := cli.NewContext(&cli.App{Writer: os.Stdout}, set, nil)
				err := cmd.CmdKeys.Run(context)
				if (err != nil) != tt.wantErr {
					t.Errorf("CmdKeys.Run() error = %v, wantErr %v", err, tt.wantErr)
				}
				w.Close()
				var buf bytes.Buffer
				io.Copy(&buf, r)
				commandOutput := buf.String()
				if tt.expectedOutput != commandOutput {
					t.Errorf("expectedOutput: %#v, commandOutput: %#v", tt.expectedOutput, commandOutput)
				}
				//Restore stdout
				os.Stdout = realStdout
			})
		}
	})
}
