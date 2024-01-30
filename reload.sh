#!/bin/sh

ps -ef | grep hot-svr | grep -v "grep" | awk '{print $2}' | xargs kill -1