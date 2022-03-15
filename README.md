# LUKS tools

`luks-key` tool is designed to retrieve key by using custom plugins and to save it into ramdisk
for systemd-crytpsetup service to open crypted DM device.

Configuration file is located in `/etc/luks-tools/config.yml`

Available options:

```
manage_network: false
keystore_path: /root/keystore
plugin: ./plugins/file_plugin/file_plugin.so
plugin_config_file_path: path
```

`manage_network` - if defined, `luks-key` will try to bring network up and retrieve configuration via DHCP (needed if plugin uses network to retrieve key).  

`keystore_path` - path where `secret.key` file will be stored. It will be mounted automatically mounted as tmpfs.

`plugin` - defines plugin to use. See example plugin at `plugins/file_plugin`.

`plugin_config_file_path` - parameters starting with `plugin_config_` should be used to configure   plugin itself. `plugin_config_file_path` is specific to `file_plugin` and defines full path to
file containing secret string.


## vault plugins

Takes folowing configuration parameters:

`plugin_config_vault_address`
`plugin_config_vault_storage_path`
`plugin_config_vault_role_id`
`plugin_config_vault_secret_id`
`plugin_config_vault_skip_verify` - disables Vault certificate check.
`plugin_config_server_id` - unique keyname where servers key is stored.

## dmtune

Tool is designed to tune device mapper parameters to increase performance for SSD drives.
To enable `allow_discards no_read_workqueue no_write_workqueue` parameters on drive, execute:
`dmtune DM_DEVICE_NAME`
List of DM devices enabled for tuning should be defined in config.yml file:
```
dm_tune_drives:
  - sda
  - luks-0000000-1111-2222-3333-444444444444
```

Refs: https://blog.cloudflare.com/speeding-up-linux-disk-encryption/

## kdump-collector

Acts as SSH server. Uploads received files (via pipe or SCP) to S3 endpoint.

Configuration is done via env variables:

`KDUMP_STRIP_FROM_NAME` - removes string from beginning of the upload path. Upload path is used to
build bucket name in s3. Normally `kdump` uploads crashdumps to `/var/crash/hostname-date/`.
`/var/crash` is not unique, so can be omitted.
default: `/var/crash/`

`KDUMP_SSH_AUTHORIZED_KEYS_FILE` path to authorized_keys file.
default: `authorized_keys`

`KDUMP_SSH_PRIVATE_KEY_FILE` path to SSH private key file.
default: `private.key`

`KDUMP_SSH_BIND_ADDRESS` bind address for `kdump-collector` SSH service.
default: `0.0.0.0:22`

`KDUMP_S3_CHUNK_SIZE` chunk size for S3 uploads. Files, bigger than chunk size will be uploaded via multipart upload.
default: `536870912` (512MB)

`KDUMP_READ_BUFF_SIZE` buffer size for ssh channel reads.
default: `1024`

`KDUMP_S3_ACCESS_KEY_ID` S3 access key id.

`KDUMP_S3_SECRET_KEY` S3 secret key.

`KDUMP_S3_ENDPOINT` S3 endpoint.

`KDUMP_S3_EREGION` S3 region.
default: `us-east-1`

`KDUMP_S3_FORCE_PATH_STYLE` Enable path-style S3 URLs.
default: `true`
