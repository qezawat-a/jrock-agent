package main

import (
	"github.com/yourname/jrock-agent/internal/agent"
	"github.com/yourname/jrock-agent/internal/config"
)

func main() { agent.New(config.Load()).Run() }
