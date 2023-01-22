import { observable } from "mobx";

export enum Resource {
    Wood = 'wood',
    Sulfur = 'sulfur',
    Crystal = 'crystal',
    Mercury = 'mercury',
    Ore = 'ore',
    Gems = 'gems',
    Gold = 'gold',
}

export const resources = observable({
    [Resource.Wood]: 20,
    [Resource.Sulfur]: 10,
    [Resource.Crystal]: 10,
    [Resource.Mercury]: 10,
    [Resource.Ore]: 20,
    [Resource.Gems]: 10,
    [Resource.Gold]: 2000,
});
