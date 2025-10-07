# Feature: Element Search and Navigation

## Problem Description

The current application lacks the ability to efficiently search for and navigate between elements of a specific type within the DOM. Users need a way to:

1. Search for elements of a specific type (e.g., `h1`, `button`) using a CSS selector.
2. Build and maintain an internal list of all matching elements in the current DOM.
3. Navigate to the first element in this list.
4. Provide navigation capabilities to move to the first, next, previous, or last element in the list.

## Objectives

- Implement a command to search for elements using a CSS selector and store the results in an internal list.
- Develop navigation commands to allow users to move through the list of elements.
- Ensure the feature is flexible, allowing users to define the type of element they wish to search for.

## Considerations

- The feature should integrate seamlessly with existing navigation and interaction commands.
- Performance should be optimized to handle large DOMs efficiently.
- User feedback should be clear, indicating the current position within the list and any errors encountered.

## User Story: MCP Client Search via Roderik MCP Server

**As** an MCP client operator using the roderik MCP server  
**I want** to capture a session where DuckDuckGo is queried for “Martin Spernau books and publications”  
**So that** future teammates understand the end-to-end workflow and tool behavior in the MCP context.

### Scenario: Execute DuckDuckGo search and record outcomes
**Given** the user asked through the MCP client “please go to duckduckgo and search for Martin Spernau's books and publications”  
**When** the operator (acting via the roderik MCP server tooling) loads `https://duckduckgo.com/`, focuses the main search field, and types the query text  
**And** clicking the search button submits the form after an Enter keypress fails to trigger submission  
**Then** the search results page is available for inspection and conversion to Markdown for reporting back through the MCP client.

### Notes
- Loading DuckDuckGo and typing the query worked as expected.  
- Submitting by newline did not trigger the search, so the search button was clicked instead.  
- Re-running the Markdown conversion provided the full results after an initial empty response.
