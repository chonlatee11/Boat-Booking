#!/usr/bin/env bash
# proto/pii-check.sh [dir]
#
# Fails (exit 1) if any *.proto file under dir (default: proto/events)
# declares a field whose name looks like personal data. Events carry ids
# only — PII travels via sync call, never in the Kafka payload (D-45).
set -euo pipefail

dir="${1:-proto/events}"

# Field name blacklist, matched as a whole identifier immediately followed
# by "=" (a proto field assignment: `string email = 1;`). Case-insensitive.
pattern='\b(email|e_mail|phone|mobile|tel|first_name|last_name|full_name|given_name|family_name|passport|national_id|citizen_id|id_card|address|birth|dob)\b[[:space:]]*='

matches=$(grep -rniE --include='*.proto' "$pattern" "$dir" 2>/dev/null || true)

if [ -n "$matches" ]; then
  echo "PII-shaped field name(s) found in proto events:" >&2
  echo "$matches" >&2
  exit 1
fi

exit 0
