# sample-data

This folder shows how to:

- Setup an admin connection to a vandrare server

- Create new configuration files to interact with a vandrare server

- Manual registration of a new client

## How to use

Call `make gen-all` to setup everything from scratch, all you need is:

- A running instance of `vandrare ssh gateway` whose admin-public-key is `~/.ssh/config/id_ed21559.pub`,
you can change this key via `identityKey=<path to private key>`

- The instance must be availabe at `localhost:8222`

## Other relevant targets

- `make init`: runs the initial setup, without the final config

- `make gen-jumpserver`: creates the jumpserver-config for administration purposes

- `make gen-server1-example-com`: creates a configuration for a client that exposes `server1.example.com`,
this configuration is done via the admin interface without the need for an initial key-registration process.

    To check how the key-registration process works, check the `new-key-registration` folder.
