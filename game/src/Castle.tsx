import React from 'react';

import { inject, observer } from "mobx-react";
import { Resource } from "./Store";

function CastlePage() {
  return (
    <div className="castle">
      <div className="castle_layout">
        <div className="row">
          <Building title="Mage Guild" description="Arcane" owned={true}/>
          <Building title="Marketplace" description="Purchase wares" owned={false}/>
          <Building title="Blacksmith" description="Upgrade equipment" owned={true}/>
        </div>
        <div className="row">
          <Building title="Scout Post" description="Improve scouting" owned={false}/>
          <Building title="Inn" description="Recover" owned={true}/>
          <Building title="Church" description="Heal" owned={true}/>
        </div>
        <div className="row">
          <Building title="Training Grounds" description="Train heroes" owned={true}/>
          <Building title="Barracks" description="Update party" owned={true}/>
          <Building title="Tavern" description="Recruit Heroes" owned={true}/>
        </div>
      </div>
      <div className="dashboard">
        <div className="dashboard_left">
          <div className="party">
            <Hero name="Foo"/>
            <Hero name="Bar"/>
            <Hero name="Simone"/>
          </div>
        </div>
        <div className="dashboard_right">
          <div className="resources">
            <div className="row">
              <ResourceDisplay resource={Resource.Wood} />
              <ResourceDisplay resource={Resource.Sulfar}/>
            </div>
            <div className="row">
              <ResourceDisplay resource={Resource.Crystal}/>
              <ResourceDisplay resource={Resource.Mercury}/>
            </div>
            <div className="row">
              <ResourceDisplay resource={Resource.Ore}/>
              <ResourceDisplay resource={Resource.Gems}/>
            </div>
            <div className="row">
              <ResourceDisplay resource={Resource.Gold}/>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}

interface ResourceDisplayProps {
  resource: Resource;
  resources?: any;
}

@inject('resources')
@observer
class ResourceDisplay extends React.PureComponent<ResourceDisplayProps> {
  render() {
    const { resource, resources } = this.props;
    return (
      <div className={`resource ${resource}`}>
        <div className="resource_icon">{resource}</div>
        <span className="resource_count">{resources[resource]}</span>
      </div>
    )
  }
}

interface BuildingProps {
  title: string;
  description: string;
  owned: boolean;
  resources?: any;
}

@inject('resources')
@observer
class Building extends React.PureComponent<BuildingProps> {
  handleClick = () => {
    const { resources } = this.props;
    resources.Gold -= 100;
  };

  render() {
    const { description, title, owned  } = this.props;

    return (
      <div className={`building ${owned ? 'owned' : 'unowned'}`}>
        <button onClick={this.handleClick} className="placeholder" title={description}></button>
        <div className="title">
          <span>{title}</span>
        </div>
      </div>
    )
  }
}

interface HeroProps {
  name: string;
}

function Hero(props: HeroProps) {
  return <div className="hero">{props.name}</div>
}

export default CastlePage;