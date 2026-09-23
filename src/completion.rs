use std::io;
use std::sync::LazyLock;

use clap::{Command, ValueEnum};

#[derive(Copy, Clone, Debug, PartialEq, Eq, ValueEnum)]
#[allow(clippy::enum_variant_names)]
pub enum Shell {
    Bash,
    Zsh,
    Fish,
    // Spelled as one word, matching how the shell itself is invoked.
    #[value(name = "powershell")]
    PowerShell,
}

pub fn generate(shell: Shell, cmd: &mut Command) {
    let name = cmd.get_name().to_string();
    let mut out = io::stdout();
    let generator = match shell {
        Shell::Bash => clap_complete::Shell::Bash,
        Shell::Zsh => clap_complete::Shell::Zsh,
        Shell::Fish => clap_complete::Shell::Fish,
        Shell::PowerShell => clap_complete::Shell::PowerShell,
    };
    clap_complete::generate(generator, cmd, name, &mut out);
}

/// Help text shown for `tag completion --help`, mirroring the shell-specific
/// installation instructions documented for each shell.
pub static LONG_HELP: LazyLock<String> = LazyLock::new(|| {
    let name = "tag";
    format!(
        "To load completions:

Bash:

  $ source <({name} completion bash)

  # To load completions for each session, execute once:
  # Linux:
  $ {name} completion bash > /etc/bash_completion.d/{name}
  # macOS:
  $ {name} completion bash > $(brew --prefix)/etc/bash_completion.d/{name}

Zsh:

  # If shell completion is not already enabled in your environment,
  # you will need to enable it.  You can execute the following once:

  $ echo \"autoload -U compinit; compinit\" >> ~/.zshrc

  # To load completions for each session, execute once:
  $ {name} completion zsh > \"${{fpath[1]}}/_{name}\"

  # You will need to start a new shell for this setup to take effect.

fish:

  $ {name} completion fish | source

  # To load completions for each session, execute once:
  $ {name} completion fish > ~/.config/fish/completions/{name}.fish

PowerShell:

  PS> {name} completion powershell | Out-String | Invoke-Expression

  # To load completions for every new session, run:
  PS> {name} completion powershell > {name}.ps1
  # and source this file from your PowerShell profile.
"
    )
});
