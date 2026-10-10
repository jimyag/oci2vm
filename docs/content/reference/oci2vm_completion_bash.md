## oci2vm completion bash

Generate the autocompletion script for bash

### Synopsis

Generate the autocompletion script for the bash shell.

This script depends on the 'bash-completion' package.
If it is not installed already, you can install it via your OS's package manager.

To load completions in your current shell session:

	source <(oci2vm completion bash)

To load completions for every new session, execute once:

#### Linux:

	oci2vm completion bash > /etc/bash_completion.d/oci2vm

#### macOS:

	oci2vm completion bash > $(brew --prefix)/etc/bash_completion.d/oci2vm

You will need to start a new shell for this setup to take effect.


```
oci2vm completion bash
```

### Options

```
  -h, --help              help for bash
      --no-descriptions   disable completion descriptions
```

### Options inherited from parent commands

```
      --time string   Enable formated timed output, valide formats: 'relative (rel | r)', 'full (f)' (default "none")
  -v, --verbose       Enable Verbose output
```

### SEE ALSO

* [oci2vm completion](oci2vm_completion.md)	 - Generate the autocompletion script for the specified shell

