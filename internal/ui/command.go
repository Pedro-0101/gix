package ui

import "strings"

type Command interface {
	Name() string
	Execute(args []string) bool
}

type CommandRegistry struct {
	commands map[string]Command
}

func newCommandRegistry() *CommandRegistry {
	return &CommandRegistry{commands: make(map[string]Command)}
}

func (r *CommandRegistry) Register(cmd Command) {
	r.commands[cmd.Name()] = cmd
}

func (r *CommandRegistry) Execute(input string) bool {
	if !strings.HasPrefix(input, "/") {
		return false
	}
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return false
	}
	name := strings.TrimPrefix(parts[0], "/")
	cmd, ok := r.commands[name]
	if !ok {
		return false
	}
	return cmd.Execute(parts[1:])
}

type newCommand struct{}

func (c *newCommand) Name() string { return "new" }

func (c *newCommand) Execute(_ []string) bool {
	newConversation()
	return true
}
