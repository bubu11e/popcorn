// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (c) 2024-2026 Julien Girard

package push

import webpush "github.com/SherClockHolmes/webpush-go"

// GenerateVAPIDKeys returns a fresh base64url-encoded VAPID key pair. The public
// key is shared with browsers to subscribe; the private key signs push requests
// and must be kept secret. Used by the `-genvapid` bootstrap flag.
func GenerateVAPIDKeys() (privateKey, publicKey string, err error) {
	return webpush.GenerateVAPIDKeys()
}
