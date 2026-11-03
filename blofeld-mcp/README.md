# Motivation

This is an experimental repository. Waldorf Blofeld is a great synthesizer, but the user interface is quite complex. There are several patch editors, even hardware solutions (dedicated controllers). I wanted to try something different. Why not use an LLM to create a patch? It doesn't need to be a perfect solution. A better "randomizer" will be good enough. 

# Demo


[![Demo](https://img.youtube.com/vi/2UmedI6hOUk/0.jpg)](https://www.youtube.com/watch?v=2UmedI6hOUk)

## Build 

You'll need the GoLang SDK to build the code.

Build: 

```
go build  .
```


## Setup 

Edit `claude_desktop_config.json` and add `blofeldcmp`:

```
{
    "mcpServers": {

        "blofeldmcp": {
            "command": "/Users/youruser/blofeld-mcp/blofeldmcp",
            "args": ["mcp"]
        }
    }
}

```


## Debug helpers
- Test notes: `./blofeldmcp play`
- Single sound test: `./blofeldmcp single`
- Dump a patch: `./blofeldmcp get`
- Load a patch: `./blofeldmcp set`


