import unreal

names = [n for n in dir(unreal) if "Exporter" in n or "Export" in n]
print("EXPORTERS", names)
mesh = unreal.load_asset(
    "/ProceduralVegetationEditor/SampleAssets/StarterContent/ConiferTree_01/Instances/Conifer_Twig_01"
)
for cls_name in ("StaticMeshExporterOBJ", "ObjExporter"):
    cls = getattr(unreal, cls_name, None)
    if cls is None:
        print("no", cls_name)
        continue
    task = unreal.AssetExportTask()
    task.object = mesh
    task.filename = f"/workspace/ue/out/debug/twig_{cls_name}.obj"
    task.exporter = cls()
    task.automated = True
    task.replace_identical = True
    task.prompt = False
    try:
        ok = unreal.Exporter.run_asset_export_task(task)
        print("EXPORT", cls_name, ok, task.filename)
    except Exception as e:
        print("EXPORT FAILED", cls_name, e)
