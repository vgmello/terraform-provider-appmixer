# Security Policy

## Supported Versions

We actively support the following versions with security updates:

| Version | Supported          |
| ------- | ------------------ |
| 0.0.x   | :white_check_mark: |

## Reporting a Vulnerability

We take security vulnerabilities seriously. If you discover a security vulnerability in this provider, please report it responsibly.

### How to Report

1. **DO NOT** create a public GitHub issue for security vulnerabilities
2. Email security reports to: [github@vgmello.com](mailto:github@vgmello.com)
3. Include the following information:
   - Description of the vulnerability
   - Steps to reproduce the issue
   - Potential impact assessment
   - Any suggested fixes or mitigations

### Response Timeline

- **Initial Response**: Within 24-48 hours of receiving your report
- **Status Update**: Weekly updates on progress
- **Resolution**: We aim to resolve critical vulnerabilities within 7-14 days

### Disclosure Policy

- We will acknowledge receipt of your vulnerability report
- We will provide regular updates on our progress
- We will credit you in our security advisory (unless you prefer to remain anonymous)
- We follow responsible disclosure practices and will coordinate public disclosure timing with you

## Security Best Practices for Provider Users

### Credential Management

- **Never commit credentials** (API URL, username, password) to version control
- Use environment variables (`APPMIXER_BASE_URL`, `APPMIXER_USERNAME`, `APPMIXER_PASSWORD`) or a secrets manager
- Regularly rotate admin credentials; use `appmixer_user` password rotation support in Terraform
- Use a dedicated admin account for Terraform automation rather than personal credentials

### State File Security

- Terraform state contains sensitive values (passwords, tokens) in plaintext
- Use a remote backend with encryption at rest (e.g. Terraform Cloud, S3 with SSE, Azure Blob with CMK)
- Restrict access to state files to only those who need it
- Never commit `.tfstate` files to version control (already in `.gitignore`)

### Provider Configuration

- Mark password variables as `sensitive = true` in your Terraform configuration
- Use `terraform.tfvars` files only for non-sensitive values; use environment variables or a vault for secrets
- Audit provider version pins (`version = "~> 0.0"`) and update regularly

## Security Contacts

- **Maintainer**: [github@vgmello.com](mailto:github@vgmello.com)
- **GitHub Repository**: https://github.com/vgmello/terraform-provider-appmixer

---

**Last Updated**: April 2026
