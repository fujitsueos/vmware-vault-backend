# vmware-vault-backend

It is a custom plugin for acquiring vmware token using vault without exposing any credentials to other applications

## Installation and usage:

**It is required to have golang installed on your system**

1. To install plugin you first need to clone the repository:
    ```
    go get git@github.com:fujitsueos/vmware-vault-backend.git
    ```
2. Build binary:
    ```
    cd $GOPATH/github.com/fujitsueos/vmware-vault-backend
    go build . # this will place vmware-vault-backend binary in current directory and can be changed using `-o` option
    ```
3. Calculate backend binary checksum:
    ```
    SHASUM=$(shasum -a 256 "<generate_binary_file_path>" | cut -d " " -f1)
    ```
4. Create vault config.hcl with path to `plugin_directory` which should be used for custom plugins eg.:
    ```yaml
    listener "tcp" {
        address     = "127.0.0.1:8200"
        tls_disable = 1
    }
    plugin_directory = "/vault/plugins"
    ```
5. Start vault server using created config.hcl:
    ```
    vault server -config config.hcl
    ```
6. Register plugin:
    ```
    vault write sys/plugins/catalog/secret/<BACKEND_NAME> \
        sha_256="<SHASUM>" \
        command="<BACKEND_NAME>"
    ```
7. Enable plugin:
   ```
   vault secrets enable -path=vmware/<subscription_id> -plugin-name=<BACKEND_NAME> plugin
   ```
8. Configure subscription credentials:
    ```
    vault write vmware/<subscription_id>/config \ 
        authentication_url=<auth_url> \
        api_url=<api_url> \
        password=<password> \
        username=<username> \
        region=<region>
    ```
9. Acquire token:
    ```
    vault read vmware/<subscription_id>/token
    ```

## Plugin development

Because of vmware rules it is possible to test vmware plugin only from inside of GCP cluster.

To test changes done in plugin the easiest way is to use tool called [draft](https://github.com/Azure/draft).

When it is properly configured (some changes in `draft.toml` file are needed, like setting proper `namespace`) and initialized it is enough to use `draft up` command, that would create new deployment of vmware-vault-backend in cluster, start development server and automatically register plugin for which `./scripts/docker_dev.sh` is used.
