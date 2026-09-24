# Install as a systemd service

Follow [Run your first job](../tutorial/first-job.md) to prepare Forgejo,
Incus, a VM image, and a runner checksum. This guide installs FARM on a Linux
host with systemd. Commands may vary by distribution.

Build from the repository root:

```sh
make build
sudo useradd --system --create-home --home-dir /var/lib/farm \
  --shell /usr/sbin/nologin farm
sudo install -d -o root -g farm -m 0750 /etc/farm
sudo install -d -o farm -g farm -m 0750 /var/lib/farm
sudo install -o root -g root -m 0755 dist/farm /usr/local/bin/farm
sudo install -o root -g farm -m 0640 farm.example.yaml /etc/farm/farm.yaml
sudo install -o root -g root -m 0644 packaging/farm.service \
  /etc/systemd/system/farm.service
sudo install -o root -g farm -m 0640 /dev/null /etc/farm/forgejo-token
sudoedit /etc/farm/forgejo-token
```

Edit `/etc/farm/farm.yaml`. Set the Forgejo URL and token path, runner URL
and digest, Incus endpoint and project, and each pool's scope, labels, image,
and profiles. The example uses `/var/lib/farm/farm.db` and
`/etc/farm/forgejo-token`.

Validate and start the service:

```sh
sudo -u farm /usr/local/bin/farm validate -config /etc/farm/farm.yaml
sudo systemctl daemon-reload
sudo systemctl enable --now farm
sudo systemctl status farm
curl --fail http://127.0.0.1:8080/healthz
```

Validation checks the configuration file. Startup also checks access to token
and certificate files and connects to Forgejo and Incus. If startup fails,
check `sudo journalctl -u farm -f`.
