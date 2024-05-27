#!/bin/sh -e
#
# SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

while :; do
  /bin/shoot-client
  openvpn --config /openvpn.config
  sleep 1
done
