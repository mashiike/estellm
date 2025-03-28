local projectRoot = std.extVar('projectRoot');
{
  mcpServers:{
    "filesystem": {
      command: "npx",
      args: [
        "-y",
        "@modelcontextprotocol/server-filesystem",
        projectRoot+"/docs"
      ],
    },
    "fetch": {
      command: "uvx",
      args: [
        "mcp-server-fetch"
      ],
    },
  },
}
