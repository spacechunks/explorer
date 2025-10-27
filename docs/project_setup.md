# Project setup

Now, we will cover how you can set up your project, so it's ready to be published to the Chunk Explorer! At the moment, 
publishing is only possible using our CLI tool. With it, you will be able to perform all important operations regarding 
Chunks and Flavors, like creating, updating and so on. So, make sure you have it installed!

## Configuration

The CLI works by looking for a config file called `.chunk.yaml` containing all important configuration options for your
Chunk. Below is a sample config:

```yaml
version: v1alpha1
chunk:
  # The name of your chunk (50-character limit)
  name: MyChunk
  # Describe your chunks in a few words (100-character limit)
  description: this is a description
  # You can have up to 5 tags categorizing your Chunk (25-character limit)
  tags:
    - tag1
    - tag2
  flavors:
      # The name of one of your flavors (25-character limit)
    - name: flavor1
      # The version of your flavor (25-character limit)
      version: v1
      # The Minecraft version your Flavor runs on
      minecraft_version: 1.21.8
      # The path to the directory where your Minecraft server
      # configuration lives. Currently, only Paper is supported.
      path: ./my_chunk/flavor1
```

## Sample project directory layout

Here is a sample layout of a BedWars minigame, which demonstrates how you could structure your project. If you have different
requirements regarding your directory layout, you can simply specify different paths.

```
bedwars
├── 8x1
│  ├── bukkit.yml
│  ├── commands.yml
│  ├── plugins
│  │  ├── bedwars
│  │  │  └── config.yaml
│  │  └── bedwars.jar
│  ├── server.properties
│  ├── spigot.yml
│  └── world
├── 8x4
│   ├── bukkit.yml
│   ├── commands.yml
│   ├── plugins
│   │  ├── bedwars
│   │  │  └── config.yaml
│   │  └── bedwars.jar
│   ├── server.properties
│   ├── spigot.yml
│   └── world
└── .chunk.yaml
```

The config file for this layout would look like the following

```yaml
version: v1alpha1
chunk:
  name: BedWars
  description: Simple BedWars minigame
  tags:
    - pvp
    - bedwars
  flavors:
    - name: 8x1
      version: v1
      minecraft_version: 1.21.8
      path: ./8x1
    - name: 8x4
      version: v1
      minecraft_version: 1.21.8
      path: ./8x4
```

