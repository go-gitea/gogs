#!/bin/bash

# Prepare git folder
mkdir -p ${HOME} && chmod 0700 ${HOME}
if [ ! -w ${HOME} ]; then echo "${HOME} is not writable"; exit 1; fi

# Prepare custom folder
mkdir -p ${GITEA_CUSTOM} && chmod 0500 ${GITEA_CUSTOM}

# Prepare temp folder
mkdir -p ${GITEA_TEMP} && chmod 0700 ${GITEA_TEMP}
if [ ! -w ${GITEA_TEMP} ]; then echo "${GITEA_TEMP} is not writable"; exit 1; fi

#Prepare config file
if [ ! -f ${GITEA_APP_INI} ]; then

    #Prepare config file folder
    GITEA_APP_INI_DIR=$(dirname ${GITEA_APP_INI})
    mkdir -p ${GITEA_APP_INI_DIR} && chmod 0700 ${GITEA_APP_INI_DIR}
    if [ ! -w ${GITEA_APP_INI_DIR} ]; then echo "${GITEA_APP_INI_DIR} is not writable"; exit 1; fi

    # Set INSTALL_LOCK to true only if SECRET_KEY is not empty and
    # INSTALL_LOCK is empty
    if [ -n "$SECRET_KEY" ] && [ -z "$INSTALL_LOCK" ]; then
        INSTALL_LOCK=true
    fi

    # Substitude the environment variables in the template
    APP_NAME=${APP_NAME:-"Gitea: Git with a cup of tea"} \
    RUN_MODE=${RUN_MODE:-"prod"} \
    RUN_USER=${USER:-"git"} \
    SSH_DOMAIN=${SSH_DOMAIN:-"localhost"} \
    HTTP_PORT=${HTTP_PORT:-"3000"} \
    ROOT_URL=${ROOT_URL:-""} \
    DISABLE_SSH=${DISABLE_SSH:-"false"} \
    SSH_PORT=${SSH_PORT:-"2222"} \
    SSH_LISTEN_PORT=${SSH_LISTEN_PORT:-$SSH_PORT} \
    DB_TYPE=${DB_TYPE:-"sqlite3"} \
    DB_HOST=${DB_HOST:-"localhost:3306"} \
    DB_NAME=${DB_NAME:-"gitea"} \
    DB_USER=${DB_USER:-"root"} \
    DB_PASSWD=${DB_PASSWD:-""} \
    INSTALL_LOCK=${INSTALL_LOCK:-"false"} \
    DISABLE_REGISTRATION=${DISABLE_REGISTRATION:-"false"} \
    REQUIRE_SIGNIN_VIEW=${REQUIRE_SIGNIN_VIEW:-"false"} \
    SECRET_KEY=${SECRET_KEY:-""} \
    envsubst < /etc/templates/app.ini > ${GITEA_APP_INI}
fi

# Replace app.ini settings with env variables in the form GITEA__SECTION_NAME__KEY_NAME
environment-to-ini --config ${GITEA_APP_INI}

# Create first admin user if need be
# Conditions:
# * GITEA_ADMIN_USER and GITEA_ADMIN_EMAIL set
# * no users exist already
# If GITEA_ADMIN_PASSWORD is set, use it; else generate a random passord
gitea=/usr/local/bin/gitea
gitea_ini="${GITEA_APP_INI}"
if [ -n "$GITEA_ADMIN_USER" ] && [ -n "$GITEA_ADMIN_EMAIL" ]; then
  # Waiting for database to be online and migrate
  while true; do
    $gitea -c "${gitea_ini}" migrate && break
    sleep 5
  done
  users="$($gitea -c "$gitea_ini" admin user list)"
  if [ -z "$(echo "$users" | tail -n +3)" ]; then
    if [ -n "$GITEA_ADMIN_PASSWORD" ]; then
      set_password=( --password "$GITEA_ADMIN_PASSWORD" )
    else
      set_password=( --random-password )
    fi
    $gitea -c "$gitea_ini" admin user create --admin --username "$GITEA_ADMIN_USER" --email "$GITEA_ADMIN_EMAIL" "${set_password[@]}"
  fi
fi
