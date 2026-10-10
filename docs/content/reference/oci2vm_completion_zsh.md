## oci2vm completion zsh

Generate the autocompletion script for zsh

### Synopsis

Generate the autocompletion script for the zsh shell.

If shell completion is not already enabled in your environment you will need
to enable it.  You can execute the following once:

	echo "autoload -U compinit; compinit" >> ~/.zshrc

To load completions in your current shell session:

	source <(oci2vm completion zsh)

To load completions for every new session, execute once:

#### Linux:

	oci2vm completion zsh > "${fpath[1]}/_oci2vm"

#### macOS:

	oci2vm completion zsh > $(brew --prefix)/share/zsh/site-functions/_oci2vm

You will need to start a new shell for this setup to take effect.


```
oci2vm completion zsh [flags]
```

### Options

```
  -h, --help              help for zsh
      --no-descriptions   disable completion descriptions
```

### Options inherited from parent commands

```
      --time string   Enable formated timed output, valide formats: 'relative (rel | r)', 'full (f)' (default "none")
  -v, --verbose       Enable Verbose output
```

### SEE ALSO

* [oci2vm completion](oci2vm_completion.md)	 - Generate the autocompletion script for the specified shell

