# Use an image-provided runner

Use this mode when the VM image already contains Forgejo Runner and its
dependencies. Set the pool's installation mode:

```yaml
instance:
  image: farm-nixos
  runner_installation: image
```

If every pool uses this mode, omit the global `runner.download_url` and
`runner.sha256` fields. FARM does not inject cloud-init. It waits for the Incus
agent, writes `/etc/farm/runner.yml`, and restarts `forgejo-runner.service`.

The image must provide:

- systemd and a working Incus agent
- a `runner` user and group
- `/etc/farm` owned by `runner:runner`, writable by the runner, mode `0700`
- the runner binary, its dependencies, and `forgejo-runner.service`

The service must execute `forgejo-runner -c /etc/farm/runner.yml one-job --wait`.
Use `ConditionPathExists=/etc/farm/runner.yml`; do not start it at boot or
bake `runner.yml` into the image. FARM supplies the ephemeral credentials and
starts the service.

For example, a NixOS image can provide the runner service with:

```nix
{pkgs, ...}:
{
  nix.settings.experimental-features = [ "nix-command" "flakes" ];

  environment.systemPackages = with pkgs; [ forgejo-runner ];

  users.groups.runner = {};
  users.users.runner = {
    isSystemUser = true;
    group = "runner";
    home = "/var/lib/forgejo-runner";
    createHome = true;
  };

  systemd.tmpfiles.rules = [ "d /etc/farm 0700 runner runner -" ];

  systemd.services.forgejo-runner = {
    description = "Forgejo Actions Runner";
    after = [ "network-online.target" ];
    wants = [ "network-online.target" ];
    unitConfig.ConditionPathExists = "/etc/farm/runner.yml";

    serviceConfig = {
      Type = "simple";
      User = "runner";
      Group = "runner";
      WorkingDirectory = "/var/lib/forgejo-runner";
      ExecStart = "${pkgs.forgejo-runner}/bin/forgejo-runner -c /etc/farm/runner.yml one-job --wait";
      Restart = "on-failure";
      RestartSec = 5;
    };

    path = with pkgs; [ bash coreutils git nix ];
    environment.NIX_PATH = "nixpkgs=${pkgs.path}";
  };
}
```

For a controller-installed runner, see [runner installation in the
configuration reference](../reference/configuration.md#runner-installation).
