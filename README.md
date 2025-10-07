Roderik is a command-line tool that allows you to navigate, inspect, and interact with elements on a webpage. It uses the Go Rod library for web scraping and automation. You can use it to walk through the DOM, get information about elements, and perform actions like clicking or typing.

Usage:
  roderik [flags]
  roderik [command]

Available Commands:
  body        Navigate to the document's body
  box         Get the box of the current element
  child       Navigate to the first child of the current element
  click       Click on the current element
  completion  Generate the autocompletion script for the specified shell
  elem        Navigate to the first element that matches the CSS selector
  head        Navigate to the first heading of the specified level, or any level if none is specified
  help        Help about any command
  html        Print the HTML of the current element
  next        Navigate to the next element
  parent      Navigate to the parent of the current element
  prev        Navigate to the previous element
  rclick      Right click on the current element
  text        Print the text of the current element
  type        Type text into the current element
  walk        Walk to the next element for a number of steps

Flags:
  -h, --help   help for roderik

Use "roderik [command] --help" for more information about a command.

##Status
As of now, this is very muc a WiP.
It kida already works, with most basic interaction and inspection commands present.
It very much needs refining and better error handling etc.

Recent reliability tweaks:
- Heading discovery (`head`, initial load) now evaluates the DOM through an inline function, preventing Rod's cached helper from occasionally disappearing and halting navigation.
- Multiple `roderik` instances can now run side by side by falling back to disposable Chrome user-data profiles when the shared profile is locked, avoiding singleton panics.

## Similar

This is a successor to my earlier attempt, called willbrowser, which was written in nodejs and the playwright framework. https://github.com/gurucoyote/willbrowser
