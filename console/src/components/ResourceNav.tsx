import * as React from 'react';
import { NavLink } from 'react-router-dom';
import { Nav, NavItem, NavList } from '@patternfly/react-core';

const ResourceNav: React.FC = () => (
  <Nav theme="light">
    <NavList>
      <NavItem>
        <NavLink to="/trex-console/projects" activeClassName="pf-m-current">
          Projects
        </NavLink>
      </NavItem>
      <NavItem>
        <NavLink to="/trex-console/entity-definitions" activeClassName="pf-m-current">
          Entity Definitions
        </NavLink>
      </NavItem>
      <NavItem>
        <NavLink to="/trex-console/field-definitions" activeClassName="pf-m-current">
          Field Definitions
        </NavLink>
      </NavItem>
      <NavItem>
        <NavLink to="/trex-console/relationships" activeClassName="pf-m-current">
          Relationships
        </NavLink>
      </NavItem>
      <NavItem>
        <NavLink to="/trex-console/builds" activeClassName="pf-m-current">
          Builds
        </NavLink>
      </NavItem>
      <NavItem>
        <NavLink to="/trex-console/dinosaurs" activeClassName="pf-m-current">
          Dinosaurs
        </NavLink>
      </NavItem>
      <NavItem>
        <NavLink to="/trex-console/fossils" activeClassName="pf-m-current">
          Fossils
        </NavLink>
      </NavItem>
      <NavItem>
        <NavLink to="/trex-console/scientists" activeClassName="pf-m-current">
          Scientists
        </NavLink>
      </NavItem>
    </NavList>
  </Nav>
);

export default ResourceNav;
