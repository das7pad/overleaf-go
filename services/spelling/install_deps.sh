#!/usr/bin/env bash
set -ex

export DEBIAN_FRONTEND=noninteractive

apt-get update
apt-get install \
  aspell \
  aspell-bg \
  aspell-de \
  aspell-de-1901 \
  aspell-en \
  aspell-fr \
  aspell-pt-br \
  --yes

rm -rf /var/lib/apt/lists/* \
