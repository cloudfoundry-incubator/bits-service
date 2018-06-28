#!/bin/bash -ex

brew list git-secrets || brew install git-secrets
brew list lastpass-cli || brew install lastpass-cli --with-pinentry
git secrets --install
git secrets --register-aws || echo "Could not register AWS patterns (maybe they're already in .git/config)"
git secrets --add-provider "$(cd "$(dirname "$0")" && pwd -P)"/list-lastpass-passwords.sh
