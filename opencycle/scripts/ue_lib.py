"""Unreal-side helpers for the coast ride's builders: textures, materials, actors, instancing.

Load it with importlib from a sibling script (see coast_assets.py); the scripts folder can't go
on sys.path because its inspect.py shadows the stdlib's.
"""

import os

import unreal

TEXTURES = "/workspace/ue/textures"
FOLDER = "/Game/Coast"
edit = unreal.MaterialEditingLibrary
assets = unreal.EditorAssetLibrary
tools = unreal.AssetToolsHelpers.get_asset_tools()
SHARED = unreal.SamplerSourceMode.SSM_WRAP_WORLD_GROUP_SETTINGS


def log(message):
    unreal.log(f"COAST {message}")


def fresh(name, folder, cls, factory):
    """A new asset, replacing any old one of that name."""
    path = f"{folder}/{name}"
    if assets.does_asset_exist(path):
        assets.delete_asset(path)
    made = tools.create_asset(name, folder, cls, factory)
    if made is None:
        # The delete can leave the old object in memory while something loaded still refers to
        # it, and the create then refuses the name. Reuse the old asset: the callers set every
        # property they need, and a material starts again from an empty graph.
        made = unreal.load_asset(path)
        # cls is a Python class, or for a C++ data asset the UClass from unreal.load_class.
        kind = cls if isinstance(cls, type) else None
        if made is None or (
            made.get_class() != cls if kind is None else not isinstance(made, kind)
        ):
            raise RuntimeError(f"could not replace {path}")
        if isinstance(made, unreal.Material):
            unreal.MaterialEditingLibrary.delete_all_material_expressions(made)
        unreal.log_warning(f"reusing {path}: it could not be deleted")
    return made


def import_file(path, folder):
    task = unreal.AssetImportTask()
    task.filename = path
    task.destination_path = folder
    task.replace_existing = True
    task.automated = True
    task.save = False
    tools.import_asset_tasks([task])
    return [unreal.load_asset(p) for p in (task.get_editor_property("imported_object_paths") or [])]


def texture(name, kind="colour", source=None, folder=f"{FOLDER}/Textures"):
    """A texture, imported once: colour, a normal map, or a linear mask. `source` is a file path;
    by default TEXTURES/name.jpg."""
    path = f"{folder}/{name}"
    if assets.does_asset_exist(path):
        return unreal.load_asset(path)
    imported = import_file(source or f"{TEXTURES}/{name}.jpg", folder)
    result = next(
        (a for a in imported if isinstance(a, unreal.Texture)), None
    ) or unreal.load_asset(path)
    if result is None:
        raise RuntimeError(f"could not import {source or name}")
    if kind == "normal":
        result.set_editor_property(
            "compression_settings", unreal.TextureCompressionSettings.TC_NORMALMAP
        )
        result.set_editor_property("srgb", False)
    elif kind == "mask":
        result.set_editor_property(
            "compression_settings", unreal.TextureCompressionSettings.TC_GRAYSCALE
        )
        result.set_editor_property("srgb", False)
    elif kind == "linear":
        result.set_editor_property("srgb", False)
    assets.save_loaded_asset(result, False)
    return result


def has_texture_set(source):
    return os.path.exists(f"{TEXTURES}/{source}_diff_2k.jpg")


class Graph:
    """A material being built, with shorthand for the expressions the builders use."""

    def __init__(self, name, folder=FOLDER):
        self.material = fresh(name, folder, unreal.Material, unreal.MaterialFactoryNew())
        self.name = name
        self.y = 0

    def node(self, cls, x=-600, y=None, **props):
        if y is None:
            y = self.y
            self.y += 140
        expression = edit.create_material_expression(self.material, cls, x, y)
        for key, value in props.items():
            expression.set_editor_property(key, value)
        return expression

    def link(self, a, b, pin_b="", pin_a=""):
        if not edit.connect_material_expressions(a, pin_a, b, pin_b):
            raise RuntimeError(
                f"{self.name}: could not connect {a.get_name()}.{pin_a!r} "
                f"-> {b.get_name()}.{pin_b!r}"
            )
        return b

    def const(self, value):
        if isinstance(value, (tuple, list)):
            node = self.node(unreal.MaterialExpressionConstant3Vector)
            node.set_editor_property("constant", unreal.LinearColor(*value, 1.0))
            return node
        return self.node(unreal.MaterialExpressionConstant, r=float(value))

    def op(self, cls, a, b=None, **props):
        node = self.node(cls, **props)
        self.link(a, node, "A")
        if b is not None:
            self.link(b, node, "B")
        return node

    def mul(self, a, b):
        return self.op(
            unreal.MaterialExpressionMultiply,
            a,
            b if not isinstance(b, (int, float)) else None,
            **({"const_b": float(b)} if isinstance(b, (int, float)) else {}),
        )

    def add(self, a, b):
        return self.op(
            unreal.MaterialExpressionAdd,
            a,
            b if not isinstance(b, (int, float)) else None,
            **({"const_b": float(b)} if isinstance(b, (int, float)) else {}),
        )

    def lerp(self, a, b, alpha):
        node = self.node(unreal.MaterialExpressionLinearInterpolate)
        self.link(a, node, "A")
        self.link(b, node, "B")
        self.link(alpha, node, "Alpha")
        return node

    def mask(self, a, channels):
        # Every channel explicitly: a new ComponentMask starts with R and G on.
        node = self.node(
            unreal.MaterialExpressionComponentMask, **{c: c in channels for c in "rgba"}
        )
        return self.link(a, node)

    def saturate(self, a):
        return self.link(a, self.node(unreal.MaterialExpressionSaturate))

    def world_uv(self, tile, offset=(0.0, 0.0)):
        """World XY / tile, so one repeat spans `tile` cm."""
        world = self.node(unreal.MaterialExpressionWorldPosition)
        xy = self.mask(world, "rg")
        uv = self.mul(xy, 1.0 / tile)
        if offset != (0.0, 0.0):
            shift = self.node(unreal.MaterialExpressionConstant2Vector, r=offset[0], g=offset[1])
            uv = self.add(uv, shift)
        return uv

    def world_noise(self, scale, low=0.0, high=1.0, levels=3):
        world = self.node(unreal.MaterialExpressionWorldPosition)
        noise = self.node(
            unreal.MaterialExpressionNoise,
            scale=scale,
            quality=2,
            levels=levels,
            output_min=low,
            output_max=high,
        )
        self.link(world, noise, "World Position")
        return noise

    def sample(self, tex, uv=None, kind="colour"):
        sampler = {
            "colour": unreal.MaterialSamplerType.SAMPLERTYPE_COLOR,
            "normal": unreal.MaterialSamplerType.SAMPLERTYPE_NORMAL,
            "mask": unreal.MaterialSamplerType.SAMPLERTYPE_LINEAR_GRAYSCALE,
            "linear": unreal.MaterialSamplerType.SAMPLERTYPE_LINEAR_COLOR,
            "masks": unreal.MaterialSamplerType.SAMPLERTYPE_MASKS,
        }[kind]
        node = self.node(unreal.MaterialExpressionTextureSample, x=-900)
        node.set_editor_property("texture", tex)
        node.set_editor_property("sampler_type", sampler)
        node.set_editor_property("sampler_source", SHARED)
        if uv is not None:
            self.link(uv, node, "UVs")
        return node

    def texture_set(self, source, tile, offset=(0.0, 0.0), uv=None):
        """A Poly Haven set's colour, normal and roughness at `tile` cm, as {diff, nor, rough}
        outputs."""
        uv = uv or self.world_uv(tile, offset)
        colour = self.sample(texture(f"{source}_diff_2k"), uv)
        normal = self.sample(texture(f"{source}_nor_dx_2k", "normal"), uv, "normal")
        rough = self.sample(texture(f"{source}_rough_2k", "mask"), uv, "mask")
        return {
            "diff": self.mask(colour, "rgb"),
            "nor": self.mask(normal, "rgb"),
            "rough": self.mask(rough, "r"),
        }

    def blend_sets(self, a, b, alpha):
        return {k: self.lerp(a[k], b[k], alpha) for k in a}

    def output(
        self,
        colour=None,
        normal=None,
        rough=None,
        specular=None,
        metallic=None,
        emissive=None,
        opacity_mask=None,
        opacity=None,
        subsurface=None,
        ao=None,
    ):
        props = unreal.MaterialProperty
        for value, prop in (
            (colour, props.MP_BASE_COLOR),
            (normal, props.MP_NORMAL),
            (rough, props.MP_ROUGHNESS),
            (specular, props.MP_SPECULAR),
            (metallic, props.MP_METALLIC),
            (emissive, props.MP_EMISSIVE_COLOR),
            (opacity_mask, props.MP_OPACITY_MASK),
            (opacity, props.MP_OPACITY),
            (subsurface, props.MP_SUBSURFACE_COLOR),
            (ao, props.MP_AMBIENT_OCCLUSION),
        ):
            if value is None:
                continue
            if isinstance(value, (int, float, tuple, list)):
                value = self.const(value)
            if not edit.connect_material_property(value, "", prop):
                raise RuntimeError(f"{self.name}: could not wire {prop}")

    def finish(self, **props):
        for key, value in props.items():
            self.material.set_editor_property(key, value)
        errors = edit.recompile_material(self.material)
        if errors:
            raise RuntimeError(f"{self.name} did not compile: {list(errors)}")
        assets.save_loaded_asset(self.material, False)
        log(f"material {self.name}")
        return self.material


def surface(
    name, source, tile, tint=None, rough_scale=1.0, rough_bias=0.0, specular=None, folder=FOLDER
):
    """A plain world-mapped material from one Poly Haven set."""
    g = Graph(name, folder)
    t = g.texture_set(source, tile)
    colour = t["diff"]
    if tint is not None:
        colour = g.mul(colour, g.const(tint))
    rough = t["rough"]
    if rough_scale != 1.0:
        rough = g.mul(rough, rough_scale)
    if rough_bias:
        rough = g.add(rough, rough_bias)
    g.output(colour, t["nor"], g.saturate(rough), specular)
    return g.finish()


def instance(name, parent, folder, textures=None, scalars=None, vectors=None):
    """A material instance of `parent` with parameter values."""
    mi = fresh(
        name, folder, unreal.MaterialInstanceConstant, unreal.MaterialInstanceConstantFactoryNew()
    )
    mi.set_editor_property("parent", parent)
    for key, value in (textures or {}).items():
        edit.set_material_instance_texture_parameter_value(mi, key, value)
    for key, value in (scalars or {}).items():
        edit.set_material_instance_scalar_parameter_value(mi, key, value)
    for key, value in (vectors or {}).items():
        edit.set_material_instance_vector_parameter_value(mi, key, unreal.LinearColor(*value))
    edit.update_material_instance(mi)
    assets.save_loaded_asset(mi, False)
    return mi


# --- the level --------------------------------------------------------------------------

actors = unreal.get_editor_subsystem(unreal.EditorActorSubsystem)
subobjects = unreal.get_engine_subsystem(unreal.SubobjectDataSubsystem)


def spawn(cls, where=(0, 0, 0), rotation=(0, 0, 0), label=None, folder=None):
    """`rotation` is (pitch, yaw, roll) in degrees."""
    pitch, yaw, roll = rotation
    actor = actors.spawn_actor_from_class(
        cls, unreal.Vector(*where), unreal.Rotator(roll, pitch, yaw)
    )
    if label:
        actor.set_actor_label(label)
    if folder:
        actor.set_folder_path(folder)
    return actor


def mesh_actor(
    mesh,
    where=(0, 0, 0),
    yaw=0.0,
    scale=1.0,
    label=None,
    folder=None,
    materials=None,
    pitch=0.0,
    roll=0.0,
):
    actor = spawn(unreal.StaticMeshActor, where, (pitch, yaw, roll), label, folder)
    component = actor.static_mesh_component
    component.set_static_mesh(mesh)
    if isinstance(scale, (tuple, list)):
        actor.set_actor_scale3d(unreal.Vector(*scale))
    elif scale != 1.0:
        actor.set_actor_scale3d(unreal.Vector(scale, scale, scale))
    for index, material in (materials or {}).items():
        component.set_material(index, material)
    return actor


class Instances:
    """Instanced static meshes grouped under one actor per mesh, for anything placed in
    thousands."""

    def __init__(self, folder):
        self.folder = folder
        self.components = {}
        self.pending = {}

    def add(self, mesh, where, yaw=0.0, scale=1.0, pitch=0.0, roll=0.0):
        if isinstance(scale, (int, float)):
            scale = (scale, scale, scale)
        self.pending.setdefault(mesh.get_path_name(), (mesh, []))[1].append(
            unreal.Transform(
                unreal.Vector(*where), unreal.Rotator(roll, pitch, yaw), unreal.Vector(*scale)
            )
        )

    def commit(self, cull=None, shadows=True, materials=None):
        """Builds the components. `cull` is a draw distance in cm; `materials` maps mesh path ->
        {slot: material}."""
        total = 0
        for path, (mesh, transforms) in self.pending.items():
            actor = spawn(
                unreal.Actor,
                label=f"{self.folder.split('/')[-1]}_{mesh.get_name()}",
                folder=self.folder,
            )
            root = subobjects.k2_gather_subobject_data_for_instance(actor)[0]
            params = unreal.AddNewSubobjectParams(
                parent_handle=root,
                new_class=unreal.HierarchicalInstancedStaticMeshComponent,
                blueprint_context=None,
            )
            handle, _fail = subobjects.add_new_subobject(params)
            data = unreal.SubobjectDataBlueprintFunctionLibrary.get_data(handle)
            component = unreal.SubobjectDataBlueprintFunctionLibrary.get_object(data)
            component.set_static_mesh(mesh)
            component.set_editor_property("cast_shadow", shadows)
            if cull:
                component.set_editor_property("instance_end_cull_distance", int(cull))
            for index, material in ((materials or {}).get(path) or {}).items():
                component.set_material(index, material)
            component.add_instances(transforms, False, True)
            total += len(transforms)
        log(f"instances {self.folder}: {total} of {len(self.pending)} meshes")
        self.pending = {}
        return total
