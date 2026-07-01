# OEM and VPS Distribution

Mobazha Community Edition may be distributed in a device, an appliance image,
a VPS marketplace image, a container image, or a pre-installed server. This
document defines the minimum expectations for those distributions. It is a
technical and project-policy guide, not legal advice.

## What may be distributed

An OEM or VPS provider may package the Community Edition standalone deployment
and sell hardware, installation, hosting, support, or other services around it.
The Community Edition is not a new edition when packaged this way: its enabled
payment capabilities remain limited to BTC, BCH, and LTC.

The following do not belong in a Community distribution:

- unapproved payment capabilities or provider credentials;
- platform tenancy, billing, control-plane, or private operations code;
- private relay, commercial deployment, or internal support tooling;
- pre-generated seed phrases, private keys, administrator passwords, customer
  data, or hidden remote-control credentials.

## License and source obligations

Mobazha-authored Community source is licensed under MPL-2.0. A distributor
must preserve the applicable license and copyright notices and make the Source
Code Form of MPL-covered software and its distributed modifications available
as required by MPL-2.0. OpenBazaar-derived material retains its separate MIT
notice in `LICENSES/MIT-OpenBazaar.txt`.

An appliance or marketplace image may include independent proprietary files,
provided that it continues to meet the obligations for MPL-covered files. A
distributor should obtain its own legal advice for its specific product and
jurisdiction.

## Required release material

Every certified or reference distribution must ship, or clearly link to, all
of the following material for its exact version:

1. `LICENSE`, `NOTICE`, the OpenBazaar MIT notice, and applicable third-party
   notices;
2. `SOURCE_OFFER.md` that names the canonical source repository, release tag
   or source commit, and how recipients obtain corresponding source;
3. an SPDX or CycloneDX SBOM;
4. checksums and provenance for the image, binary, or installer;
5. the exact `community.json` capability manifest; and
6. upgrade, backup, restore, factory-reset, and data-export instructions.

The source repository check can be run with:

```bash
./scripts/community/check-oem-distribution.sh --source
```

After a release bundle is assembled, validate it with:

```bash
./scripts/community/check-oem-distribution.sh --artifact /path/to/release-bundle
```

## Security and user control

Reference distributions must generate seller identity and administrator secrets
on the user's device during first-run setup. They must not send seed phrases,
private keys, order contents, or customer data to Mobazha or an OEM service.

Automated updates must identify their signing key and release channel. Users
must be able to inspect the installed version, export their data, make a local
backup, restore it, and choose not to use a managed update channel.

## Network and optional services

A local standalone store must remain usable for its local administration,
listings, data export, and Community UTXO payment flow without a required
Mobazha Hosting account. Discovery, buyer identity, search, routing, managed
updates, and support services may be offered as optional integrations only.
Their endpoint, data handling, and disablement path must be disclosed to the
operator.

The first Community release does not define an official OEM directory,
certification program, or mandatory network registry. Do not claim that an
image is "Official" or "Mobazha Certified" without a separate written
trademark authorization.

## Branding and certification

MPL-2.0 grants rights to the source code, not Mobazha names, logos, or an
official-certification claim. See `TRADEMARKS.md` and
`docs/community/ATTRIBUTION.md`.

An eventual certification program will verify reproducible source provenance,
the required release material, Community capability boundaries, secure first
run, update behavior, and recovery behavior. It is a quality and support
service, not a condition for running or joining the Community network.
