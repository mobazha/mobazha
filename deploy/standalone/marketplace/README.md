# Mobazha VPS Marketplace Images

Pre-built VM images for one-click deployment on popular VPS providers.

## Supported Providers

| Provider       | Method          | Status     |
|----------------|-----------------|------------|
| DigitalOcean   | Packer + API    | Planned    |
| Vultr          | Packer + API    | Planned    |
| Linode         | StackScript     | Planned    |
| Hetzner        | Packer + API    | Planned    |

## How It Works

1. `cloud-init.yml` pre-installs Docker, downloads Mobazha compose files, and pre-pulls the Docker image
2. On first SSH login, users see a banner directing them to run `mobazha-setup`
3. `mobazha-setup` offers interactive or token-based setup

## Building Images

### Using Packer (DigitalOcean example)

```bash
export DIGITALOCEAN_TOKEN="your-api-token"
packer build -var "do_token=$DIGITALOCEAN_TOKEN" packer-digitalocean.pkr.hcl
```

### Using StackScript (Linode)

Upload the content of `cloud-init.yml` as a Linode StackScript.

## User Experience

1. User creates a droplet/instance from the Mobazha marketplace image
2. SSH into the server
3. Run `mobazha-setup`
4. Follow the prompts or paste a deploy token from the SaaS Deploy Wizard
5. Store is live in ~2 minutes (Docker image is pre-pulled)
