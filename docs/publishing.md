# Publishing your Chunk

Before you can publish your Chunk make sure you meet these requirements:
- Have the CLI installed
- Have a project ready and set up. If not see [Project Setup](./project_setup.md) to get an introduction of how to do it.

Once you have everthing ready, head over to your directory where your project lives and execute

```
explorer chunk publish
```

this will create a Chunk and Flavor, if they don't already exist, as well as creating an execution plan, that will give 
insights on what will happen if you proceed with publishing.

For example, it will show what flavors are new, what files where added, changed or removed since the last version,
if there are any other actions, like file uploads that need to be done and so on.

Publishing for the first time will lead to an execution plan that looks something like this:

```
New flavors:
 MyFlavor:
  + Version:  v1
  + Path:     ./chunk/flavor1
  + Files:
    +  banned-ips.json
    +  banned-players.json
    +  bukkit.yml
    +  commands.yml
    +  config/paper-global.yml
    +  config/paper-world-defaults.yml
    +  eula.txt
    +  help.yml
    +  ops.json
    +  permissions.yml
    +  plugins/bStats/config.yml
    +  plugins/spark/config.json
    +  plugins/spark/tmp-client/about.txt
    +  server.properties
    +  spigot.yml
    +  world/data/chunks.dat
    +  world/data/raids.dat
    +  world/data/random_sequences.dat
    +  world/data/scoreboard.dat
    +  world/datapacks/bukkit/pack.mcmeta
    +  world/level.dat
    +  world/paper-world.yml
    +  world/region/r.-1.-1.mca
    +  world/region/r.-1.0.mca
    +  world/region/r.0.-1.mca
    +  world/region/r.0.0.mca
    +  world/stats/92de217b-8b2b-403b-86a5-fe26fa3a9b5f.json
    +  world/uid.dat

Are you sure you want to publish? (y/n):
```

It shows all files that have been added as well as information about the flavor. This output will be repeated, if there
are more flavors present. Entering `y` will start the publishing process.

## Retry publishing on errors

As with all things, errors are something to expect, so, if, for example, the server is not available or a build step
failed, you can simply execute the command again and the execution plan will show you the action that will be performed
for the flavor that failed to be published.

```
Actions to be performed for the following flavors:
 MyFlavor => Retry uploading files

Are you sure you want to publish? (y/n):
```

The following actions can be retried:
- File upload failed
- Image building failed
- Checkpoint building failed
