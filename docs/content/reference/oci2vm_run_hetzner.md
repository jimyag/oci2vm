## oci2vm run hetzner

Run the virtual machine image on Hetzner Cloud

```
oci2vm run hetzner [options] image-path [flags]
```

### Options

```
  -h, --help              help for hetzner
  -l, --location string   oci2vm server location (default "hel1-dc2")
  -n, --name string       oci2vm server name (default "oci2vm")
      --rm                remove server when done
  -i, --ssh-key string    oci2vm image identity key
      --token string      Hetzner Cloud API token [$HETZNER_TOKEN]
  -t, --type string       oci2vm server type (default "cx11")
  -u, --user string       oci2vm image ssh user (default "root")
```

### Options inherited from parent commands

```
      --time string   Enable formated timed output, valide formats: 'relative (rel | r)', 'full (f)' (default "none")
  -v, --verbose       Enable Verbose output
```

### SEE ALSO

* [oci2vm run](oci2vm_run.md)	 - Run the virtual machine image

