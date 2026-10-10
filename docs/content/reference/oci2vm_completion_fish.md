## oci2vm completion fish

Generate the autocompletion script for fish

### Synopsis

Generate the autocompletion script for the fish shell.

To load completions in your current shell session:

	oci2vm completion fish | source

To load completions for every new session, execute once:

	oci2vm completion fish > ~/.config/fish/completions/oci2vm.fish

You will need to start a new shell for this setup to take effect.


```
oci2vm completion fish [flags]
```

### Options

```
  -h, --help              help for fish
      --no-descriptions   disable completion descriptions
```

### Options inherited from parent commands

```
      --time string   Enable formated timed output, valide formats: 'relative (rel | r)', 'full (f)' (default "none")
  -v, --verbose       Enable Verbose output
```

### SEE ALSO

* [oci2vm completion](oci2vm_completion.md)	 - Generate the autocompletion script for the specified shell

