import * as React from 'react';
import { Label, Spinner } from '@patternfly/react-core';

type StatusColor = 'grey' | 'blue' | 'green' | 'orange' | 'red';

const PROJECT_STATUS_COLORS: Record<string, StatusColor> = {
  draft: 'grey',
  active: 'green',
  archived: 'orange',
};

const BUILD_STATUS_COLORS: Record<string, StatusColor> = {
  pending: 'grey',
  building: 'blue',
  succeeded: 'green',
  failed: 'red',
};

type Props = {
  status: string;
  type: 'project' | 'build';
};

const StatusBadge: React.FC<Props> = ({ status, type }) => {
  const colorMap = type === 'project' ? PROJECT_STATUS_COLORS : BUILD_STATUS_COLORS;
  const color = colorMap[status] || 'grey';

  return (
    <Label color={color} icon={status === 'building' ? <Spinner size="sm" /> : undefined}>
      {status}
    </Label>
  );
};

export default StatusBadge;
