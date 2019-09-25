#!/usr/bin/env bash

AppName="$1"
RollbackInSec=$(($2))

function init2() {
  if [ ! "${AppName}" ]; then
    echo "app name not specify"
    return 1
  fi
  echo "set app name: ${AppName}"
  if [ ! -f "${AppName}" ]; then
    echo "specified app , not exist"
    return 2
  fi
  if [ $RollbackInSec -le 0 ]; then
    RollbackInSec=3
  fi
  echo "set rollback duration: ${RollbackInSec}"
  return 0
}

if ! init2; then
  echo "init script failed"
  exit 1
fi

echo "init script success"

RollbackInSec=3

# dont set it
UpdateName="${AppName}.update"
BakName="${AppName}.bak"

function doRollback() {
  if [ ! -f "${BakName}" ]; then
    return 1
  fi
  rm -f "${UpdateName}"
  mv "${BakName}" "${AppName}"
  return 0
}

function doUpdate() {
  if [ -f "${UpdateName}" ]; then
    echo "update found!"
    mv "${AppName}" "${BakName}"
    mv "${UpdateName}" "${AppName}"
    echo "repalce complete!"
  else
    echo "app no update."
  fi
}

doUpdate

echo "run app..."

chmod +x "${AppName}"

timeStamp1=$(date +%s)

# run it
./"${AppName}"

appExitCode=$?

timeStamp2=$(date +%s)

timeDiff=$((timeStamp2 - timeStamp1))

if [ $appExitCode -ne 0 ] && [ $timeDiff -lt $RollbackInSec ]; then
  # rollback
  echo "app quit in ${RollbackInSec} seconds, rollback"
  if doRollback; then
    echo "app rollback success"
  else
    echo "app rollback failed"
  fi
fi
echo "app exit (${appExitCode})"

echo "daemon exit..."
