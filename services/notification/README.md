# Notification Service

Consumes events from the **notification-events** Kafka topic and sends email (Brevo), SMS (Termii), and WhatsApp messages.

## Event format

Produce JSON messages to the `notification-events` topic with:

| Field     | Type   | Description |
|----------|--------|-------------|
| `type`   | string | e.g. `email_verification`, `sms_otp`, `whatsapp_alert`, `transfer_receipt` |
| `channel` | string | `email`, `sms`, or `whatsapp` |
| `metadata` | object | Channel-specific fields (see below) |

### Email (Brevo) – `channel: "email"`

- `to` (required): recipient email
- `subject` (required for non-template): subject line
- `body`: plain text body
- `html`: HTML body (use one of body/html/template_id)
- `template_id`: Brevo template ID (use with `params` for variables)
- `params`: map of template variables
- `to_name`: optional recipient name

### SMS (Termii) – `channel: "sms"`

- `to` (required): phone in E.164 (e.g. `23490126727`)
- `body` or `message` (required): SMS text
- `channel`: `generic` (default) or `dnd` (for OTP/transactional, bypasses DND)

### WhatsApp – `channel: "whatsapp"`

**Text (within 24h reply window):**

- `to` (required): phone with country code (e.g. `23490126727`)
- `body` or `message` (required): message text

**Template (any time):**

- `to` (required)
- `template_name`: approved template name
- `template_language`: e.g. `en_US`
- `template_params`: array of strings for {{1}}, {{2}}, …

## Environment variables

| Variable | Description |
|----------|-------------|
| `KAFKA_BROKER` | Kafka broker address (default `redpanda:9092`) |
| `NOTIFICATION_SERVICE_PORT` | HTTP port (default `8004`) |
| `BREVO_API_KEY` | Brevo API key |
| `BREVO_SENDER_EMAIL` | From email |
| `BREVO_SENDER_NAME` | From name |
| `TERMII_API_KEY` | Termii API key |
| `TERMII_SENDER_ID` | Alphanumeric sender ID (3–11 chars) |
| `TERMII_BASE_URL` | Termii API base (default `https://api.termii.com`) |
| `WHATSAPP_ACCESS_TOKEN` | WhatsApp Business Cloud API token |
| `WHATSAPP_PHONE_NUMBER_ID` | WhatsApp Business phone number ID |
| `WHATSAPP_API_VERSION` | Graph API version (default `v21.0`) |

Omitting a provider’s keys disables that channel (no crash).
