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
